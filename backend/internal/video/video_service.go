package video

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"

	"gorm.io/gorm"
)

type VideoService struct {
	repo         *VideoRepository
	cache        *rediscache.Client
	cacheTTL     time.Duration
	popularityMQ *rabbitmq.PopularityMQ
}

func NewVideoService(repo *VideoRepository, cache *rediscache.Client, popularityMQ *rabbitmq.PopularityMQ) *VideoService {
	return &VideoService{repo: repo, cache: cache, cacheTTL: 5 * time.Minute, popularityMQ: popularityMQ}
}

func (vs *VideoService) Publish(ctx context.Context, video *Video) error {
	if video == nil {
		return errors.New("video is nil")
	}
	video.Title = strings.TrimSpace(video.Title)
	video.PlayURL = strings.TrimSpace(video.PlayURL)
	video.CoverURL = strings.TrimSpace(video.CoverURL)

	if video.Title == "" {
		return errors.New("title is required")
	}
	if video.PlayURL == "" {
		return errors.New("play url is required")
	}
	if video.CoverURL == "" {
		return errors.New("cover url is required")
	}

	//事务保证视频写入库和消息写入本地消息表的一致性
	err := vs.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil {
			return err
		}

		msg := OutboxMsg{
			VideoID:    video.ID,
			EventType:  "video_published",
			Status:     "pending",
			CreateTime: video.CreateTime,
		}

		if err := tx.Create(&msg).Error; err != nil {
			return err
		}

		tags := ExtractTags(video.Title + " " + video.Description)
		for _, tagName := range tags {
			var tag Tag
			if err := tx.Where("name = ?", tagName).FirstOrCreate(&tag, Tag{Name: tagName}).Error; err != nil {
				return err
			}
			if err := tx.Create(&VideoTag{VideoID: video.ID, TagID: tag.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return err

}

func (vs *VideoService) Delete(ctx context.Context, id uint, authorID uint) error {
	video, err := vs.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if video == nil {
		return errors.New("video not found")
	}
	if video.AuthorID != authorID {
		return app.ErrUnauthorized
	}
	if err := vs.repo.DeleteVideo(ctx, id); err != nil {
		return err
	}
	if vs.cache != nil {
		cacheKey := vs.cache.Key("video:detail:id=%d", id)
		_ = vs.cache.Del(context.Background(), cacheKey)
	}
	return nil
}

func (vs *VideoService) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	videos, err := vs.repo.ListByAuthorID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (vs *VideoService) GetDetail(ctx context.Context, id uint) (*Video, error) {
	if vs.cache == nil {
		return vs.repo.GetByID(ctx, id)
	}

	readCache := func(ctx context.Context, key string) (*Video, error, bool) {
		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		b, err := vs.cache.GetBytes(opCtx, key)
		if err != nil {
			if rediscache.IsMiss(err) {
				return nil, nil, false
			}
			return nil, err, true
		}
		var video Video
		if err := json.Unmarshal(b, &video); err != nil {
			return nil, nil, false
		}
		return &video, nil, true
	}

	loadAndCache := func(ctx context.Context, id uint, key string) (*Video, error) {
		video, err := vs.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if b, err := json.Marshal(video); err == nil {
			opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			defer cancel()
			_ = vs.cache.SetBytes(opCtx, key, b, vs.cacheTTL)
		}
		return video, nil
	}

	cacheKey := vs.cache.Key("video:detail:id=%d", id)
	lockKey := "lock:" + cacheKey

	if video, err, ok := readCache(ctx, cacheKey); ok {
		return video, err
	}

	lockCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	token, locked, _ := vs.cache.Lock(lockCtx, lockKey, 2*time.Second)
	cancel()

	if locked {
		defer vs.cache.Unlock(context.Background(), lockKey, token)

		if video, err, ok := readCache(ctx, cacheKey); ok {
			return video, err
		}

		return loadAndCache(ctx, id, cacheKey)
	}

	for i := 0; i < 5; i++ {
		time.Sleep(20 * time.Millisecond)
		if video, err, ok := readCache(ctx, cacheKey); ok {
			return video, err
		}
	}
	return loadAndCache(ctx, id, cacheKey)
}

func (vs *VideoService) ListDetails(ctx context.Context, ids []uint) ([]Video, error) {
	seen := make(map[uint]bool, len(ids))
	orderedIDs := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		orderedIDs = append(orderedIDs, id)
		if len(orderedIDs) >= 50 {
			break
		}
	}
	if len(orderedIDs) == 0 {
		return []Video{}, nil
	}

	videos, err := vs.repo.ListByIDs(ctx, orderedIDs)
	if err != nil {
		return nil, err
	}

	byID := make(map[uint]Video, len(videos))
	for _, v := range videos {
		byID[v.ID] = v
	}

	ordered := make([]Video, 0, len(videos))
	for _, id := range orderedIDs {
		if v, ok := byID[id]; ok {
			ordered = append(ordered, v)
		}
	}
	return ordered, nil
}

func (vs *VideoService) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	if err := vs.repo.UpdateLikesCount(ctx, id, likesCount); err != nil {
		return err
	}
	return nil
}

func (vs *VideoService) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	if err := vs.repo.UpdatePopularity(ctx, id, change); err != nil {
		return err
	}

	if vs.popularityMQ != nil {
		if err := vs.popularityMQ.Update(ctx, id, change); err == nil {
			return nil
		}
	}

	if vs.cache != nil {
		// 1) 详情缓存：直接失效（最简单靠谱）
		opCtx_, cancel_ := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel_()
		if err := vs.cache.Del(opCtx_, vs.cache.Key("video:detail:id=%d", id)); err != nil {
			log.Printf("failed to delete video cache: %v", err)
		}

		// 2) 热榜：写到“时间窗ZSET”，不要用 detail key
		now := time.Now().UTC().Truncate(time.Minute)
		windowKey := vs.cache.Key("hot:video:1m:%s", now.Format("200601021504"))
		member := strconv.FormatUint(uint64(id), 10)

		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		_ = vs.cache.ZincrBy(opCtx, windowKey, member, float64(change))
		_ = vs.cache.Expire(opCtx, windowKey, 2*time.Hour)
	}
	return nil
}
