package feed

import (
	"PulseFeed/internal/video"
	"context"
	"time"

	"gorm.io/gorm"
)

type FeedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) *FeedRepository {
	return &FeedRepository{db: db}
}

// ListLatest 查询全站最新视频。
//
// 排序规则：
//
//	create_time DESC, id DESC
//
// beforeTime:
//
//	零值：第一页
//	非零：查询 create_time < beforeTime 的旧视频
func (repo *FeedRepository) ListLatest(
	ctx context.Context,
	limit int,
	beforeTime time.Time,
) ([]*video.Video, error) {
	var videos []*video.Video

	if limit <= 0 {
		limit = 20
	}

	query := repo.db.WithContext(ctx).
		Model(&video.Video{}).
		Order("create_time DESC").
		Order("id DESC")

	if !beforeTime.IsZero() {
		query = query.Where("create_time < ?", beforeTime)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

// ListLikesCountWithCursor 按点赞数查询视频。
//
// 排序规则：
//
//	likes_count DESC, id DESC
//
// cursor == nil：第一页
//
// cursor != nil：查询 cursor 后面的数据
//
// 下一页条件：
//
//	likes_count < cursor.likes_count
//	OR (likes_count = cursor.likes_count AND id < cursor.id)
func (repo *FeedRepository) ListLikesCountWithCursor(
	ctx context.Context,
	limit int,
	cursor *LikesCursor,
) ([]*video.Video, error) {
	var videos []*video.Video

	if limit <= 0 {
		limit = 20
	}

	query := repo.db.WithContext(ctx).
		Model(&video.Video{}).
		Order("likes_count DESC").
		Order("id DESC")

	if cursor != nil {
		query = query.Where(
			"(likes_count < ?) OR (likes_count = ? AND id < ?)",
			cursor.LikesCount,
			cursor.LikesCount,
			cursor.ID,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

// ListByFollowing 查询当前用户关注的人发布的视频。
//
// 排序规则：
//
//	create_time DESC, id DESC
//
// viewerAccountID:
//
//	当前登录用户 ID。
//	如果 viewerAccountID == 0，直接返回空列表。
//	关注流不应该在未登录时退化成全站最新流。
//
// beforeTime:
//
//	零值：第一页
//	非零：查询 create_time < beforeTime 的旧视频
func (repo *FeedRepository) ListByFollowing(
	ctx context.Context,
	limit int,
	viewerAccountID uint,
	beforeTime time.Time,
) ([]*video.Video, error) {
	var videos []*video.Video

	if limit <= 0 {
		limit = 20
	}

	if viewerAccountID == 0 {
		return []*video.Video{}, nil
	}

	followingSubQuery := repo.db.WithContext(ctx).
		Model(&social.Social{}).
		Select("vlogger_id").
		Where("followed_id = ?", viewerAccountID)

	query := repo.db.WithContext(ctx).
		Model(&video.Video{}).
		Where("author_id IN ?", followingSubQuery).
		Order("create_time DESC").
		Order("id DESC")

	if !beforeTime.IsZero() {
		query = query.Where("create_time < ?", beforeTime)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

// ListByPopularityOffset 按热度查询视频。
//
// 用途：
//
//	热门流 Redis 不可用或热榜为空时的 DB fallback。
//
// 排序规则：
//
//	popularity DESC, create_time DESC, id DESC
//
// 注意：
//
//	offset 分页简单直观，适合学习项目和兜底场景。
//	如果数据量非常大，深分页会变慢。
//	生产级热门榜更推荐 keyset cursor。
func (repo *FeedRepository) ListByPopularityOffset(
	ctx context.Context,
	limit int,
	offset int,
) ([]*video.Video, error) {
	var videos []*video.Video

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	err := repo.db.WithContext(ctx).
		Model(&video.Video{}).
		Order("popularity DESC").
		Order("create_time DESC").
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&videos).
		Error

	if err != nil {
		return nil, err
	}

	return videos, nil
}

// ListByPopularity 使用 keyset cursor 按热度查询视频。
//
// 排序规则：
//
//	popularity DESC, create_time DESC, id DESC
//
// cursor 由三部分组成：
//
//	popularityBefore
//	timeBefore
//	idBefore
//
// 下一页条件：
//
//	popularity < popularityBefore
//	OR (popularity = popularityBefore AND create_time < timeBefore)
//	OR (popularity = popularityBefore AND create_time = timeBefore AND id < idBefore)
//
// 注意：
//
//	popularity 允许为 0。
//	所以判断游标是否完整时，不能用 popularityBefore > 0。
//	只要 timeBefore 非零，并且 idBefore > 0，就认为游标有效。
func (repo *FeedRepository) ListByPopularity(
	ctx context.Context,
	limit int,
	popularityBefore int64,
	timeBefore time.Time,
	idBefore uint,
) ([]*video.Video, error) {
	var videos []*video.Video

	if limit <= 0 {
		limit = 20
	}

	query := repo.db.WithContext(ctx).
		Model(&video.Video{}).
		Order("popularity DESC").
		Order("create_time DESC").
		Order("id DESC")

	if !timeBefore.IsZero() && idBefore > 0 {
		query = query.Where(
			`(popularity < ?)
			 OR (popularity = ? AND create_time < ?)
			 OR (popularity = ? AND create_time = ? AND id < ?)`,
			popularityBefore,
			popularityBefore, timeBefore,
			popularityBefore, timeBefore, idBefore,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

// GetByIDs 根据视频 ID 批量查询视频。
//
// 用途：
//
//	Service 层从 Redis 时间线 / 热门榜中拿到 videoID 后，
//	需要根据这些 ID 查询完整的视频信息。
//
// 注意：
//
//	MySQL 的 WHERE id IN ? 不保证返回顺序和 ids 顺序一致。
//	所以 Service 层需要用 buildOrderedResult 按原始 ids 顺序重排。
func (repo *FeedRepository) GetByIDs(ctx context.Context, ids []uint) ([]*video.Video, error) {
	var videos []*video.Video
	if len(ids) == 0 {
		return videos, nil
	}

	if err := repo.db.WithContext(ctx).
		Model(&video.Video{}).
		Where("id IN ?", ids).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

// ListByTag 根据标签名查询视频。
//
// 表关系：
//
//	videos      视频表
//	tags        标签表
//	video_tags  视频和标签的中间表
//
// SQL 含义：
//
//	SELECT videos.*
//	FROM videos
//	JOIN video_tags ON video_tags.video_id = videos.id
//	JOIN tags ON tags.id = video_tags.tag_id
//	WHERE tags.name = ?
//	ORDER BY videos.create_time DESC, videos.id DESC
//	LIMIT ?;
func (repo *FeedRepository) ListByTag(
	ctx context.Context,
	tagName string,
	limit int,
	beforeTime time.Time,
) ([]*video.Video, error) {
	var videos []*video.Video

	if limit <= 0 {
		limit = 20
	}

	query := repo.db.WithContext(ctx).
		Table("videos").
		Joins("JOIN video_tags ON video_tags.video_id = videos.id").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("tags.name = ?", tagName).
		Order("videos.create_time DESC").
		Order("videos.id DESC")

	if !beforeTime.IsZero() {
		query = query.Where("videos.create_time < ?", beforeTime)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}
