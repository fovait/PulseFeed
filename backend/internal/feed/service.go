package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/video"

	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	// 本地缓存视频实体的时间。
	// 本地缓存是进程内缓存，速度快，但不同实例之间不共享，所以时间不要太长。
	localVideoEntityTTL = 5 * time.Second

	// Redis 中视频实体缓存的时间。
	// 视频实体相对稳定，可以缓存得比本地缓存久一些.
	redisVideoEntityTTL = time.Hour

	// 关注流缓存时间。
	// 关注流变化较快，比如关注的人发布新视频、取消关注、视频删除等，
	// 所以不要缓存 24 小时，短缓存更安全。
	followingFeedTTL = 30 * time.Second
)

type FeedService struct {
	// repo 负责 Feed 相关的视频查询。
	// 比如最新流、关注流、点赞榜、热门榜、标签流等。
	repo *FeedRepository

	// likeRepo 负责查询当前用户是否点赞过某些视频。
	// 用来填充 FeedVideoItem.IsLiked。
	likeRepo *video.LikeRepository

	// rediscache 是 Redis 客户端。
	// 用于：
	// 1. 视频实体缓存
	// 2. 全局时间线 feed:global_timeline
	// 3. 热门榜 hot:video:...
	// 4. 关注流缓存
	// 5. 分布式锁
	rediscache *rediscache.Client

	// localcache 是进程内本地缓存，也就是 L1 缓存。
	// 访问速度比 Redis 快，但只对当前进程有效。
	localcache *cache.Cache

	// cacheTTL 是默认缓存时间。
	// 某些特殊业务可以使用自己的 TTL，比如 followingFeedTTL。
	cacheTTL time.Duration

	// requestGroup 用于 singleflight。
	// 作用是：相同 key 的并发请求，只让一个请求真正查数据库，
	// 其他请求等待这个请求的结果，从而减少数据库压力。
	requestGroup singleflight.Group
}

func NewFeedService(
	repo *FeedRepository,
	likeRepo *video.LikeRepository,
	rediscache *rediscache.Client,
) *FeedService {
	return &FeedService{
		repo:       repo,
		likeRepo:   likeRepo,
		rediscache: rediscache,

		// 默认缓存 3 秒，每 5 秒清理一次过期项。
		// 本地缓存时间短一些，可以降低数据不一致的风险。
		localcache: cache.New(3*time.Second, 5*time.Second),

		// 默认缓存时间。
		// 注意：不是所有业务都适合 24 小时缓存。
		// 比如关注流会单独使用 followingFeedTTL。
		cacheTTL: 24 * time.Hour,
	}
}

// cacheKey 统一生成缓存 key。
//
// 如果 Redis 客户端存在，就使用 rediscache.Key 生成带项目前缀的 key。
// 如果 Redis 客户端不存在，就退化成 fmt.Sprintf，避免空指针 panic。
func (f *FeedService) cacheKey(format string, args ...any) string {
	if f.rediscache != nil {
		return f.rediscache.Key(format, args...)
	}
	return fmt.Sprintf(format, args...)
}

// trimVideosForPage 用于 limit + 1 分页。
//
// 查询时多查一条：queryLimit = limit + 1。
// 如果结果数量 > limit，说明还有下一页。
// 返回给前端时，只返回前 limit 条。
func trimVideosForPage(videos []*video.Video, limit int) ([]*video.Video, bool) {
	if len(videos) > limit {
		return videos[:limit], true
	}
	return videos, false
}

// parseUintIDs 把 Redis 中取出来的字符串 videoID 转成 []uint。
//
// Redis ZSET 的 member 通常是字符串，比如 "1001"。
// Service 层需要把它转成 uint，再去查视频详情。
func parseUintIDs(idStrs []string) []uint {
	ids := make([]uint, 0, len(idStrs))
	for _, s := range idStrs {
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil || u == 0 {
			continue
		}
		ids = append(ids, uint(u))
	}
	return ids
}

// buildOrderedResult 按照 orderedIDs 的顺序组装视频结果。
//
// 为什么需要这个函数？
// 因为 Redis 时间线 / 热榜返回的是有顺序的 videoID，
// 但 MySQL 批量查询 GetByIDs 返回的顺序不一定和传入 ID 顺序一致。
//
// 所以要用 map 存数据，再按原始 ID 顺序重新组装。
func buildOrderedResult(
	orderedIDs []uint,
	dataMap map[uint]*video.Video,
) []*video.Video {
	res := make([]*video.Video, 0, len(orderedIDs))

	for _, id := range orderedIDs {
		if v, exist := dataMap[id]; exist && v != nil {
			res = append(res, v)
		}
	}

	return res
}

// GetVideoByIDs 批量获取视频实体。
//
// 设计目标：
// 1. 尽量减少 MySQL 压力
// 2. 尽量保持 Redis / 本地缓存命中
// 3. 返回顺序和传入 videoIDs 顺序一致
//
// 缓存层级：
// L1：localcache，本地进程缓存，速度最快，但只在当前进程有效
// L2：Redis，跨进程共享缓存
// L3：MySQL，最终数据源
func (f *FeedService) GetVideoByIDs(
	ctx context.Context,
	videoIDs []uint,
) ([]*video.Video, error) {
	if len(videoIDs) == 0 {
		return []*video.Video{}, nil
	}

	// videoMap 用来临时保存已经查到的视频。
	// key 是 videoID，value 是视频实体。
	//
	// 为什么不用 slice 直接 append？
	// 因为不同视频可能来自不同缓存层，返回顺序可能乱。
	// 最后要通过 buildOrderedResult 按 videoIDs 原顺序重新组装.
	videoMap := make(map[uint]*video.Video, len(videoIDs))

	// ========================
	// L1：本地缓存
	// ========================

	// missedL1 保存本地缓存没有命中的 videoID。
	missedL1 := make([]uint, 0, len(videoIDs))

	for _, id := range videoIDs {
		key := f.cacheKey("video:entity:%d", id)

		if f.localcache != nil {
			if val, found := f.localcache.Get(key); found {
				// go-cache 取出来的是 interface{}，
				// 所以需要做类型断言。
				if v, ok := val.(video.Video); ok {
					// 拷贝一份，避免外部修改缓存里的对象。
					copyV := v
					videoMap[id] = &copyV
					continue
				}
			}
		}

		// 本地缓存没命中，进入下一层 Redis。
		missedL1 = append(missedL1, id)
	}

	// 如果所有视频都在本地缓存中命中，直接按原顺序返回。
	if len(missedL1) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	// ========================
	// L2：Redis MGET
	// ========================

	// missedL2 保存 Redis 也没有命中的 videoID。
	missedL2 := make([]uint, 0, len(missedL1))

	if f.rediscache != nil {
		keys := make([]string, len(missedL1))
		for i, id := range missedL1 {
			keys[i] = f.cacheKey("video:entity:%d", id)
		}

		// Redis 是缓存层，不应该拖慢主请求。
		// 所以这里给 Redis MGET 设置较短超时。
		cacheCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		results, err := f.rediscache.MGet(cacheCtx, keys...)
		cancel()

		if err == nil {
			for i, res := range results {
				id := missedL1[i]

				// Redis 中没这个 key
				if res == nil {
					missedL2 = append(missedL2, id)
					continue
				}

				// go-redis MGet 返回的是 []interface{}。
				// 这里期望值是 string，也就是 JSON 字符串。
				str, ok := res.(string)
				if !ok {
					missedL2 = append(missedL2, id)
					continue
				}

				var v video.Video
				if err := json.Unmarshal([]byte(str), v); err != nil {
					// JSON 解析失败，说明缓存内容可能异常。
					// 不让它影响主流程，直接降级到 MySQL。
					missedL2 = append(missedL2, id)
					continue
				}

				copyV := v
				videoMap[id] = &copyV

				// Redis 命中后，顺手回写本地缓存。
				// 这样同一进程内短时间再次访问，会更快。
				if f.localcache != nil {
					f.localcache.Set(keys[i], v, localVideoEntityTTL)
				}
			}
		} else {
			// Redis 失败时，缓存层降级，不影响主业务。
			// 全部进入 MySQL 查询。
			log.Printf("FeedService.GetVideoByIDs: Redis MGet failed, fallback to DB: %v", err)
			missedL2 = missedL1
		}
	} else {
		missedL2 = missedL1
	}

	if len(missedL2) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	// ========================
	// L3：MySQL 批量查询
	// ========================
	//
	// 注意：这里建议批量查，而不是每个 ID 起一个 goroutine 单独查。
	// 原来的写法每个 miss ID 查一次数据库，请求多时反而容易放大 DB 压力。
	//
	// 这里用 singleflight 防止同一批 miss 被并发重复查库。
	// 学习项目里可以直接用 missedL2 拼 key。
	// 生产里可以先排序再 hash，避免 key 太长或顺序差异导致无法合并。

	sfKey := f.cacheKey("sf:video:entity:%v", missedL2)

	v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
		return f.repo.GetByIDs(ctx, missedL2)
	})

	if err != nil {
		return buildOrderedResult(videoIDs, videoMap), err
	}

	dbVideos, ok := v.([]*video.Video)
	if !ok {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	for _, vid := range dbVideos {
		if vid == nil {
			continue
		}

		copyV := *vid
		videoMap[copyV.ID] = &copyV

		key := f.cacheKey("video:entity:%d", copyV.ID)

		// 写入本地缓存
		if f.localcache != nil {
			f.localcache.Set(key, copyV, localVideoEntityTTL)
		}

		// 异步写入 redis
		// 这里不阻塞当前请求，因为 Redis 回写失败不应该影响 feed 返回
		if f.rediscache != nil {
			if b, err := json.Marshal(copyV); err == nil {
				go func(key string, b []byte) {
					setCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					defer cancel()

					_ = f.rediscache.SetBytes(setCtx, key, b, redisVideoEntityTTL)
				}(key, b)
			}
		}
	}

	return buildOrderedResult(videoIDs, videoMap), nil
}

// ListLatest 查询全站最新视频流。
//
// 参数：
//
//	limit：每页数量
//	beforeTime：分页游标，表示查询 create_time < beforeTime 的视频
//	viewerAccountID：当前访问用户 ID，未登录时为 0
//
// 设计：
//  1. Redis 保存全局最新时间线 feed:global_timeline
//  2. ZSET member = videoID
//  3. ZSET score = video.CreateTime.Unix()
//  4. Redis 查不到或不可用时，降级查 MySQL
//  5. 如果 Redis 时间线为空，尝试从 MySQL 重建最近 1000 条
func (f *FeedService) ListLatest(
	ctx context.Context,
	limit int,
	beforeTime time.Time,
	viewerAccountID uint,
) (ListLatestResponse, error) {
	limit = NormalizeLimit(limit)

	// 多查一条，用于判断 has_more。
	queryLimit := limit + 1

	// 如果 Redis 没有初始化，直接走 MySQL。
	if f.rediscache == nil {
		return f.listLatestFromDB(ctx, queryLimit, limit, beforeTime, viewerAccountID)
	}

	timelineKey := f.cacheKey("feed:global_timeline")

	// Redis ZSET 默认按 score 从小到大排序。
	// rank=0 是当前 Redis 时间线里最老的一条热数据。
	zsetTail, err := f.rediscache.ZRangeWithScores(ctx, timelineKey, 0, 0)
	if err != nil {
		// Redis 查询失败，降级查 MySQL。
		return f.listLatestFromDB(ctx, queryLimit, limit, beforeTime, viewerAccountID)
	}

	// 如果 Redis 时间线为空，尝试重建。
	if len(zsetTail) == 0 {
		rebuilt, err := f.rebuildGlobalTimeline(ctx, timelineKey)
		if err != nil {
			return ListLatestResponse{}, err
		}

		// MySQL 也没有数据，直接返回空列表。
		if !rebuilt {
			return ListLatestResponse{
				VideoList:      []FeedVideoItem{},
				NextBeforeTime: 0,
				HasMore:        false,
			}, nil
		}

		// 重建成功后，重新走一遍 ListLatest。
		return f.ListLatest(ctx, limit, beforeTime, viewerAccountID)
	}

	// watermark 是 Redis 热数据中最老一条视频的时间。
	// 如果请求游标时间 <= watermark，说明用户已经翻到冷数据区域，需要查 MySQL。
	watermark := int64(zsetTail[0].Score)

	// reqTime 表示这次请求的游标时间。
	// 第一页 beforeTime 是零值，就用当前时间作为上界。
	reqTime := time.Now().Unix()
	if !beforeTime.IsZero() {
		reqTime = beforeTime.Unix()
	}

	var baseVideos []*video.Video

	if reqTime <= watermark {
		// ========================
		// 冷数据：查 MySQL
		// ========================
		//
		// 用户已经翻过 Redis 热数据范围，直接从数据库继续查。
		// 这里用 singleflight 防止多个相同冷数据请求同时打到 MySQL。

		sfKey := f.cacheKey("sf:cold:listLatest:limit=%d:before=%d", queryLimit, reqTime)

		v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
			return f.repo.ListLatest(ctx, queryLimit, beforeTime)
		})
		if err != nil {
			return ListLatestResponse{}, err
		}

		baseVideos = v.([]*video.Video)
	} else {
		// ========================
		// 热数据：查 Redis 时间线
		// ========================

		maxScore := "+inf"
		if !beforeTime.IsZero() {
			// -1 是为了防止下一页重复出现上一页最后一条。
			maxScore = fmt.Sprintf("%d", reqTime-1)
		}

		// 从 Redis ZSET 按 score 倒序取 videoID。
		// 也就是按发布时间从新到旧取。
		idStrs, err := f.rediscache.ZRevRangeByScore(
			ctx,
			timelineKey,
			maxScore,
			"-inf",
			0,
			int64(queryLimit),
		)
		if err != nil {
			return f.listLatestFromDB(ctx, queryLimit, limit, beforeTime, viewerAccountID)
		}

		videoIDs := parseUintIDs(idStrs)

		if len(videoIDs) > 0 {
			baseVideos, err = f.GetVideoByIDs(ctx, videoIDs)
			if err != nil {
				return ListLatestResponse{}, err
			}
		}

		// ========================
		// 热冷边界拼接
		// ========================
		//
		// 如果 Redis 查出来的视频不足一页，说明可能刚好碰到热冷边界。
		// 这时从 MySQL 再补一些旧视频，保证一页尽量填满。
		if len(baseVideos) < queryLimit {
			remainLimit := queryLimit - len(baseVideos)

			var coldCursor time.Time
			if len(baseVideos) > 0 {
				// 从当前页最后一条视频之后继续查旧数据。
				coldCursor = baseVideos[len(baseVideos)-1].CreateTime
			} else {
				coldCursor = beforeTime
			}

			sfKey := f.cacheKey(
				"sf:stitch:listLatest:limit=%d:before=%d",
				remainLimit,
				coldCursor.Unix(),
			)

			v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
				return f.repo.ListLatest(ctx, remainLimit, coldCursor)
			})
			if err == nil {
				coldVideos := v.([]*video.Video)
				baseVideos = append(baseVideos, coldVideos...)
			}
		}
	}

	// limit + 1 分页判断。
	pageVideos, hasMore := trimVideosForPage(baseVideos, limit)

	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListLatestResponse{}, err
	}

	var nextBeforeTime int64
	if len(pageVideos) > 0 {
		nextBeforeTime = pageVideos[len(pageVideos)-1].CreateTime.Unix()
	}

	return ListLatestResponse{
		VideoList:      feedItems,
		NextBeforeTime: nextBeforeTime,
		HasMore:        hasMore,
	}, nil
}

// rebuildGlobalTimeline 从 MySQL 重建 Redis 全局最新时间线。
//
// 只重建最近 1000 条视频，避免把大量冷数据塞进 Redis。
//
// 返回值：
//
//	true：重建成功，并且 MySQL 中有数据
//	false：MySQL 中也没有数据
func (f *FeedService) rebuildGlobalTimeline(
	ctx context.Context,
	timelineKey string,
) (bool, error) {
	sfKey := f.cacheKey("sf:rebuild:feed:global_timeline")

	v, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
		// 从 MySQL 取最新 1000 条。
		dbVideos, err := f.repo.ListLatest(ctx, 1000, time.Time{})
		if err != nil {
			return false, err
		}

		if len(dbVideos) == 0 {
			return false, nil
		}

		zs := make([]redis.Z, 0, len(dbVideos))

		for _, vid := range dbVideos {
			if vid == nil {
				continue
			}

			zs = append(zs, redis.Z{
				// 统一使用 Unix 秒级时间戳。
				Score: float64(vid.CreateTime.Unix()),

				// Redis ZSET 的 member 用字符串保存 videoID。
				Member: fmt.Sprintf("%d", vid.ID),
			})
		}

		if len(zs) == 0 {
			return false, nil
		}

		// 重建时间线属于后台修复操作。
		// 使用 Background，避免请求取消导致重建中断。
		// 但必须加超时，避免 Redis 卡住。
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := f.rediscache.ZAdd(bgCtx, timelineKey, zs...); err != nil {
			return false, err
		}

		return true, nil
	})

	if err != nil {
		return false, err
	}

	rebuilt, ok := v.(bool)
	if !ok {
		return false, nil
	}

	return rebuilt, nil
}

// listLatestFromDB 是最新流的 MySQL 兜底版本。
//
// 使用场景：
//  1. Redis 未初始化
//  2. Redis 查询失败
//  3. 用户翻到冷数据区域
func (f *FeedService) listLatestFromDB(
	ctx context.Context,
	queryLimit int,
	limit int,
	beforeTime time.Time,
	viewerAccountID uint,
) (ListLatestResponse, error) {
	videos, err := f.repo.ListLatest(ctx, queryLimit, beforeTime)
	if err != nil {
		return ListLatestResponse{}, err
	}

	pageVideos, hasMore := trimVideosForPage(videos, limit)

	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListLatestResponse{}, err
	}

	var nextBeforeTime int64
	if len(pageVideos) > 0 {
		nextBeforeTime = pageVideos[len(pageVideos)-1].CreateTime.Unix()
	}

	return ListLatestResponse{
		VideoList:      feedItems,
		NextBeforeTime: nextBeforeTime,
		HasMore:        hasMore,
	}, nil
}

// ListByLikes 按点赞数查询视频列表。
//
// 排序规则：
//
//	likes_count DESC, id DESC
//
// 为什么需要 cursor？
//
//	因为 likes_count 可能重复。
//	如果只用 likes_count 分页，可能会漏掉点赞数相同的视频。
//	所以这里用 likes_count + id 组成稳定游标。
//
// 第一页：
//
//	cursor == nil
//
// 下一页：
//
//	cursor = 上一页最后一个视频的 likes_count 和 id
func (f *FeedService) ListByLikes(
	ctx context.Context,
	limit int,
	cursor *LikesCursor,
	viewerAccountID uint,
) (ListByLikesResponse, error) {
	limit = NormalizeLimit(limit)

	// 多查一条，用来判断是否还有下一页。
	queryLimit := limit + 1

	// 从 MySQL 查询点赞榜。
	//
	// repo.ListLikesCountWithCursor 内部建议实现为：
	//
	// 第一页：
	//   SELECT *
	//   FROM videos
	//   ORDER BY likes_count DESC, id DESC
	//   LIMIT ?
	//
	// 下一页：
	//   SELECT *
	//   FROM videos
	//   WHERE likes_count < ?
	//      OR (likes_count = ? AND id < ?)
	//   ORDER BY likes_count DESC, id DESC
	//   LIMIT ?
	videos, err := f.repo.ListLikesCountWithCursor(ctx, queryLimit, cursor)
	if err != nil {
		return ListByLikesResponse{}, err
	}

	// limit + 1 分页。
	// 如果 videos 数量超过 limit，说明还有下一页。
	pageVideos, hasMore := trimVideosForPage(videos, limit)

	// 把 video.Video 转成 FeedVideoItem。
	// 这里会补充作者信息、点赞数、当前用户是否点赞过。
	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListByLikesResponse{}, err
	}

	resp := ListByLikesResponse{
		VideoList: feedItems,
		HasMore:   hasMore,
	}

	// 只有还有下一页时，才返回 next_cursor。
	//
	// next_cursor 使用当前页最后一个视频的位置。
	// 前端下一次请求把这个 cursor 原样传回来即可。
	if hasMore && len(pageVideos) > 0 {
		last := pageVideos[len(pageVideos)-1]

		resp.NextCursor = &LikesCursor{
			LikesCount: last.LikesCount,
			ID:         last.ID,
		}
	}

	return resp, nil
}

// ListByFollowing 查询关注流。
//
// 关注流是强用户态数据，viewerAccountID 必须 > 0。
// Handler 层已经做了登录校验，这里再防御一次。
//
// 分页规则：
//
//	create_time DESC
//
// cursor：
//
//	beforeTime 表示查询 create_time < beforeTime 的视频。
//	beforeTime.IsZero() 表示第一页。
//
// 缓存策略：
//
//	关注流可以短缓存，但不适合长缓存。
//	因为关注关系、视频发布、视频删除都会影响关注流。
//	这里使用 followingFeedTTL = 30s。
func (f *FeedService) ListByFollowing(
	ctx context.Context,
	limit int,
	beforeTime time.Time,
	viewerAccountID uint,
) (ListByFollowingResponse, error) {
	limit = NormalizeLimit(limit)
	queryLimit := limit + 1

	// 关注流必须登录。
	// 如果没有用户 ID，直接返回空列表或者也可以返回业务错误。
	if viewerAccountID == 0 {
		return ListByFollowingResponse{
			VideoList:      []FeedVideoItem{},
			NextBeforeTime: 0,
			HasMore:        false,
		}, nil
	}

	// 如果 Redis 可用，先尝试读关注流缓存。
	if f.rediscache != nil {
		before := int64(0)
		if !beforeTime.IsZero() {
			before = beforeTime.Unix()
		}

		cacheKey := f.cacheKey(
			"feed:following:account=%d:limit=%d:before=%d",
			viewerAccountID,
			limit,
			before,
		)

		// 1. 先查缓存
		if resp, ok := f.getFollowingCache(ctx, cacheKey); ok {
			return resp, nil
		}

		// 2. 缓存未命中，尝试加锁防止缓存击穿
		lockKey := "lock:" + cacheKey

		lockCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
		token, locked, _ := f.rediscache.Lock(lockCtx, lockKey, 500*time.Millisecond)
		cancel()

		if locked {
			// 拿到锁的请求负责查 DB 并回写缓存。
			defer func() {
				_ = f.rediscache.Unlock(context.Background(), lockKey, token)
			}()

			// 双查缓存。
			// 可能在你拿锁前，别的请求已经写入缓存。
			if resp, ok := f.getFollowingCache(ctx, cacheKey); ok {
				return resp, nil
			}

			resp, err := f.listByFollowingFromDB(
				ctx,
				queryLimit,
				limit,
				beforeTime,
				viewerAccountID,
			)
			if err != nil {
				return ListByFollowingResponse{}, err
			}

			f.setFollowingCache(cacheKey, resp)
			return resp, nil
		}

		// 3. 没拿到锁，说明可能有其他请求正在查 DB 并回填缓存。
		// 当前请求短暂等待，然后再次尝试读缓存。
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return ListByFollowingResponse{}, ctx.Err()
			case <-time.After(20 * time.Millisecond):
			}

			if resp, ok := f.getFollowingCache(ctx, cacheKey); ok {
				return resp, nil
			}
		}

		// 4. 等待后仍然没有缓存，自己查 DB。
		resp, err := f.listByFollowingFromDB(
			ctx,
			queryLimit,
			limit,
			beforeTime,
			viewerAccountID,
		)
		if err != nil {
			return ListByFollowingResponse{}, err
		}

		f.setFollowingCache(cacheKey, resp)
		return resp, nil
	}

	// Redis 不可用时，直接查 DB。
	return f.listByFollowingFromDB(
		ctx,
		queryLimit,
		limit,
		beforeTime,
		viewerAccountID,
	)
}

// listByFollowingFromDB 从 MySQL 查询关注流。
//
// 这个函数只负责：
// 1. 调用 Repository 查询关注用户的视频
// 2. 做 limit + 1 分页
// 3. 构造 FeedVideoItem
// 4. 返回 ListByFollowingResponse
func (f *FeedService) listByFollowingFromDB(
	ctx context.Context,
	queryLimit int,
	limit int,
	beforeTime time.Time,
	viewerAccountID uint,
) (ListByFollowingResponse, error) {
	videos, err := f.repo.ListByFollowing(
		ctx,
		queryLimit,
		viewerAccountID,
		beforeTime,
	)
	if err != nil {
		return ListByFollowingResponse{}, err
	}

	pageVideos, hasMore := trimVideosForPage(videos, limit)

	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListByFollowingResponse{}, err
	}

	var nextBeforeTime int64
	if len(pageVideos) > 0 {
		nextBeforeTime = pageVideos[len(pageVideos)-1].CreateTime.Unix()
	}

	return ListByFollowingResponse{
		VideoList:      feedItems,
		NextBeforeTime: nextBeforeTime,
		HasMore:        hasMore,
	}, nil
}

// getFollowingCache 读取关注流缓存。
//
// 返回值：
//
//	resp：缓存中的响应
//	ok：是否成功命中缓存
//
// 注意：
// Redis 是加速层，读取失败不要影响主流程。
// 所以这里失败时统一返回 ok=false。
func (f *FeedService) getFollowingCache(
	ctx context.Context,
	key string,
) (ListByFollowingResponse, bool) {
	if f.rediscache == nil || key == "" {
		return ListByFollowingResponse{}, false
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	b, err := f.rediscache.GetBytes(cacheCtx, key)
	if err != nil {
		return ListByFollowingResponse{}, false
	}

	var resp ListByFollowingResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return ListByFollowingResponse{}, false
	}

	// 防御一下，避免缓存里 video_list 是 null。
	if resp.VideoList == nil {
		resp.VideoList = []FeedVideoItem{}
	}

	return resp, true
}

// setFollowingCache 写入关注流缓存。
//
// 这里异步写 Redis，避免缓存写入拖慢当前请求。
// 即使写缓存失败，也不影响主流程。
func (f *FeedService) setFollowingCache(
	key string,
	resp ListByFollowingResponse,
) {
	if f.rediscache == nil || key == "" {
		return
	}

	if resp.VideoList == nil {
		resp.VideoList = []FeedVideoItem{}
	}

	b, err := json.Marshal(resp)
	if err != nil {
		return
	}

	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_ = f.rediscache.SetBytes(cacheCtx, key, b, followingFeedTTL)
	}()
}

// ListByPopularity 查询热门视频流。
//
// 分页方式：
//
//	cursor.as_of + cursor.offset
//
// 第一页：
//
//	cursor == nil
//
// 后续页：
//
//	使用上一页返回的 NextCursor
//
// Redis 热榜设计：
//  1. 每分钟一个热度 ZSET，例如 hot:video:1m:202606021530
//  2. 查询时合并最近 60 分钟的热度窗口
//  3. 合并结果写入 hot:video:merge:1m:{as_of}
//  4. 同一个 as_of 的分页复用同一个快照，保证翻页稳定
func (f *FeedService) ListByPopularity(
	ctx context.Context,
	limit int,
	cursor *PopularityCursor,
	viewerAccountID uint,
) (ListByPopularityResponse, error) {
	limit = NormalizeLimit(limit)

	// 多查一条，用来判断 has_more。
	queryLimit := limit + 1

	// 默认使用当前分钟作为榜单快照时间。
	asOf := time.Now().UTC().Truncate(time.Minute)

	offset := 0

	if cursor != nil {
		if cursor.AsOf > 0 {
			asOf = time.Unix(cursor.AsOf, 0).UTC().Truncate(time.Minute)
		}
		if cursor.Offset > 0 {
			offset = cursor.Offset
		}
	}

	// Redis 不可用时，直接用 MySQL 兜底。
	if f.rediscache == nil {
		return f.listPopularityFromDBSnapshot(
			ctx,
			queryLimit,
			limit,
			offset,
			viewerAccountID,
		)
	}

	// 确保当前 as_of 对应的热榜快照存在。
	destKey, err := f.ensurePopularitySnapshot(ctx, asOf)
	if err != nil {
		// Redis 快照构建失败，不影响主流程，降级查 DB。
		return f.listPopularityFromDBSnapshot(
			ctx,
			queryLimit,
			limit,
			offset,
			viewerAccountID,
		)
	}

	opCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()

	start := int64(offset)
	stop := start + int64(queryLimit) - 1

	// 从热榜快照里按分数倒序取 videoID。
	// ZRevRange 表示 score 从高到低。
	members, err := f.rediscache.ZRevRange(opCtx, destKey, start, stop)
	if err != nil {
		return f.listPopularityFromDBSnapshot(
			ctx,
			queryLimit,
			limit,
			offset,
			viewerAccountID,
		)
	}

	// Redis 热榜为空。
	// 对第一页来说，可以走 DB fallback；
	// 对后续页来说，说明没有更多数据。
	if len(members) == 0 {
		if offset > 0 {
			return ListByPopularityResponse{
				VideoList: []FeedVideoItem{},
				HasMore:   false,
			}, nil
		}

		return f.listPopularityFromDBSnapshot(
			ctx,
			queryLimit,
			limit,
			offset,
			viewerAccountID,
		)
	}

	videoIDs := parseUintIDs(members)

	if len(videoIDs) == 0 {
		return ListByPopularityResponse{
			VideoList: []FeedVideoItem{},
			HasMore:   false,
		}, nil
	}

	// 根据 Redis 返回的 videoID 查视频详情。
	// 这里复用 GetVideoByIDs，可以走本地缓存 / Redis 实体缓存 / MySQL。
	baseVideos, err := f.GetVideoByIDs(ctx, videoIDs)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	pageVideos, hasMore := trimVideosForPage(baseVideos, limit)

	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	resp := ListByPopularityResponse{
		VideoList: feedItems,
		HasMore:   hasMore,
	}

	// 还有下一页时，返回下一页 cursor。
	if hasMore {
		resp.NextCursor = &PopularityCursor{
			AsOf:   asOf.Unix(),
			Offset: offset + limit,
		}
	}

	return resp, nil
}

// ensurePopularitySnapshot 确保某个 as_of 对应的热门榜快照存在。
//
// 快照 key：
//
//	hot:video:merge:1m:{as_of}
//
// 构建逻辑：
//  1. 合并最近 60 个 1 分钟热榜窗口
//  2. 如果合并后有数据，直接使用
//  3. 如果合并后没有数据，则从 MySQL 按 popularity 取前 1000 条重建一个短期快照
//
// 返回值：
//
//	string：快照 key
func (f *FeedService) ensurePopularitySnapshot(
	ctx context.Context,
	asOf time.Time,
) (string, error) {
	destKey := f.cacheKey(
		"hot:video:merge:1m:%s",
		asOf.Format("200601021504"),
	)

	opCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	// 如果快照已经存在，直接复用。
	exists, _ := f.rediscache.Exists(opCtx, destKey)
	if exists {
		return destKey, nil
	}

	const windowMinutes = 60

	keys := make([]string, 0, windowMinutes)

	for i := 0; i < windowMinutes; i++ {
		key := f.cacheKey(
			"hot:video:1m:%s",
			asOf.Add(-time.Duration(i)*time.Minute).Format("200601021504"),
		)
		keys = append(keys, key)
	}

	// 合并最近 60 分钟的热度。
	//
	// 假设每个分钟窗口中：
	//   member = videoID
	//   score  = 当前分钟增加的热度
	//
	// ZUnionStore SUM 后：
	//   member = videoID
	//   score  = 最近 60 分钟总热度
	_ = f.rediscache.ZUnionStore(opCtx, destKey, keys, "SUM")

	// 快照只需要短时间存在，给翻页留一点时间即可。
	_ = f.rediscache.Expire(opCtx, destKey, 2*time.Minute)

	// 检查合并后的快照是否有数据。
	members, err := f.rediscache.ZRevRange(opCtx, destKey, 0, 0)
	if err == nil && len(members) > 0 {
		return destKey, nil
	}

	// 最近 60 分钟没有热度数据时，用 DB popularity 兜底重建一个短期快照。
	return f.rebuildPopularitySnapshotFromDB(ctx, destKey)
}

// rebuildPopularitySnapshotFromDB 从 MySQL 重建热门榜快照。
//
// 使用场景：
//  1. Redis 最近 60 分钟热榜为空
//  2. 系统刚启动，还没有热度窗口数据
//
// 这里从 DB 按 popularity 排序取前 1000 条，写入 Redis ZSET。
func (f *FeedService) rebuildPopularitySnapshotFromDB(
	ctx context.Context,
	destKey string,
) (string, error) {
	sfKey := f.cacheKey("sf:rebuild:hot:snapshot:%s", destKey)

	_, err, _ := f.requestGroup.Do(sfKey, func() (interface{}, error) {
		// 这里需要 Repository 提供一个按 popularity 排序查询的方法。
		// 下面我会给参考实现。
		dbVideos, err := f.repo.ListByPopularityOffset(ctx, 1000, 0)
		if err != nil {
			return nil, err
		}

		if len(dbVideos) == 0 {
			return nil, nil
		}

		zs := make([]redis.Z, 0, len(dbVideos))

		for _, vid := range dbVideos {
			if vid == nil {
				continue
			}

			// score 以 popularity 为主。
			// 乘一个大数后再加 create_time，用于打破 popularity 相同的排序。
			//
			// 例如：
			// popularity 高的视频一定排前面；
			// popularity 相同，则发布时间新的稍微靠前。
			score := float64(vid.Popularity)*1_000_000_000 + float64(vid.CreateTime.Unix())

			zs = append(zs, redis.Z{
				Score:  score,
				Member: fmt.Sprintf("%d", vid.ID),
			})
		}

		if len(zs) == 0 {
			return nil, nil
		}

		bgCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		if err := f.rediscache.ZAdd(bgCtx, destKey, zs...); err != nil {
			return nil, err
		}

		_ = f.rediscache.Expire(bgCtx, destKey, 2*time.Minute)

		return nil, nil
	})

	if err != nil {
		return destKey, err
	}

	return destKey, nil
}

// listPopularityFromDBSnapshot 是热门流的 MySQL 兜底版本。
//
// 注意：
// 这里为了统一前端接口，使用 offset 分页。
// 也就是说，前端热门流永远只需要维护：
//
//	cursor.as_of
//	cursor.offset
//
// 生产级高性能场景中，DB fallback 更推荐 keyset cursor：
//
//	popularity + create_time + id
//
// 但学习项目阶段，offset fallback 更容易理解和维护。
func (f *FeedService) listPopularityFromDBSnapshot(
	ctx context.Context,
	queryLimit int,
	limit int,
	offset int,
	viewerAccountID uint,
) (ListByPopularityResponse, error) {
	videos, err := f.repo.ListByPopularityOffset(ctx, queryLimit, offset)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	pageVideos, hasMore := trimVideosForPage(videos, limit)

	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	resp := ListByPopularityResponse{
		VideoList: feedItems,
		HasMore:   hasMore,
	}

	if hasMore {
		resp.NextCursor = &PopularityCursor{
			// AsOf = 0 表示当前响应来自 DB fallback，
			// 下一页继续使用 offset 即可。
			AsOf:   0,
			Offset: offset + limit,
		}
	}

	return resp, nil
}

// buildFeedVideos 把数据库中的 video.Video 转换成前端需要的 FeedVideoItem。
//
// 为什么需要这个函数？
//
// 数据库模型 video.Video 更偏向数据存储，比如：
//
//	ID
//	AuthorID
//	Username
//	Title
//	PlayURL
//	CoverURL
//	CreateTime
//	LikesCount
//	Popularity
//
// 但是 Feed 接口返回给前端时，需要的是：
//  1. 视频基础信息
//  2. 作者信息 author
//  3. 当前用户是否点赞过 is_liked
//
// 所以这里做了一层 DTO 转换。
func (f *FeedService) buildFeedVideos(
	ctx context.Context,
	videos []*video.Video,
	viewerAccountID uint,
) ([]FeedVideoItem, error) {
	if len(videos) == 0 {
		return []FeedVideoItem{}, nil
	}

	// 收集 videoID，用于批量查询当前用户是否点赞过这些视频。
	videoIDs := make([]uint, 0, len(videos))

	for _, v := range videos {
		if v == nil {
			continue
		}
		videoIDs = append(videoIDs, v.ID)
	}

	// likedMap 用来保存当前用户对视频的点赞状态。
	//
	// key: videoID
	// value: 当前用户是否点赞过
	likedMap := make(map[uint]bool)

	// 如果 viewerAccountID == 0，说明用户未登录。
	// 游客态下，不需要查点赞表，所有 is_liked 默认 false。
	if viewerAccountID != 0 && f.likeRepo != nil && len(videoIDs) > 0 {
		m, err := f.likeRepo.BatchGetLiked(ctx, videoIDs, viewerAccountID)
		if err != nil {
			return nil, err
		}
		likedMap = m
	}

	feedItems := make([]FeedVideoItem, 0, len(videos))

	for _, v := range videos {
		if v == nil {
			continue
		}

		item := FeedVideoItem{
			ID: v.ID,

			Author: FeedAuthor{
				ID:       v.AuthorID,
				Username: v.Username,
			},

			Title:       v.Title,
			Description: v.Description,
			PlayURL:     v.PlayURL,
			CoverURL:    v.CoverURL,

			// 统一使用 Unix 秒级时间戳。
			// 前面的 before_time / next_before_time 也都用秒级时间戳。
			CreateTime: v.CreateTime.Unix(),

			LikesCount: v.LikesCount,

			// 如果 likedMap 中没有这个 videoID，bool 零值就是 false。
			IsLiked: likedMap[v.ID],
		}

		feedItems = append(feedItems, item)
	}

	return feedItems, nil
}

// ListByTag 根据标签名查询视频列表。
//
// 例如前端请求：
//
//	{
//	  "tag_name": "Go",
//	  "limit": 20
//	}
//
// Service 做的事情：
//  1. 修正 limit
//  2. 查询 tagName 对应的视频
//  3. 使用 limit + 1 判断 has_more
//  4. 转换成 FeedVideoItem
func (f *FeedService) ListByTag(
	ctx context.Context,
	tagName string,
	limit int,
	beforeTime time.Time,
	viewerAccountID uint,
) (ListByTagResponse, error) {
	limit = NormalizeLimit(limit)

	// 多查一条，用于判断是否还有下一页。
	queryLimit := limit + 1

	videos, err := f.repo.ListByTag(ctx, tagName, queryLimit, beforeTime)
	if err != nil {
		return ListByTagResponse{}, err
	}

	pageVideos, hasMore := trimVideosForPage(videos, limit)

	feedItems, err := f.buildFeedVideos(ctx, pageVideos, viewerAccountID)
	if err != nil {
		return ListByTagResponse{}, err
	}

	var nextBeforeTime int64
	if len(pageVideos) > 0 {
		nextBeforeTime = pageVideos[len(pageVideos)-1].CreateTime.Unix()
	}
	return ListByTagResponse{
		VideoList:      feedItems,
		NextBeforeTime: nextBeforeTime,
		HasMore:        hasMore,
	}, nil
}
