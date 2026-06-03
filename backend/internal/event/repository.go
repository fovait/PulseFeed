package event

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EventRepository struct {
	db *gorm.DB
}

func NewEventRepository(db *gorm.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) AutoMigrate(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("event repository is not initialized")
	}
	return r.db.WithContext(ctx).AutoMigrate(&UserEvent{}, &VideoMetrics{})
}

func (r *EventRepository) FindByIdempotencyKey(ctx context.Context, key string) (*UserEvent, error) {
	if key == "" {
		return nil, ErrInvalidArgument
	}
	var evt UserEvent
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&evt).Error
	if err != nil {
		return nil, err
	}
	return &evt, nil
}

func (r *EventRepository) Save(ctx context.Context, event *UserEvent) error {
	err := r.db.WithContext(ctx).Create(event).Error
	if err == nil {
		return nil
	}
	// idempotency_key 唯一索引冲突 -> 幂等冲突，交给上层按幂等成功处理。
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDuplicate
	}
	return err
}

func (r *EventRepository) Exists(ctx context.Context, key string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&UserEvent{}).
		Where("idempotency_key = ?", key).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Increment 按事件类型对 video_metrics 的对应计数 +1，行不存在时插入（UPSERT）。
func (r *EventRepository) Increment(ctx context.Context, videoID uint, eventType EventType) error {
	if videoID == 0 {
		return ErrInvalidArgument
	}
	seed := VideoMetrics{VideoID: videoID}
	var column string
	switch eventType {
	case EventTypeImpression:
		column = "impression_count"
		seed.ImpressionCount = 1
	case EventTypeView:
		column = "view_count"
		seed.ViewCount = 1
	case EventTypePlayComplete:
		column = "play_complete_count"
		seed.PlayCompleteCount = 1
	case EventTypeShare:
		column = "share_count"
		seed.ShareCount = 1
	default:
		return ErrInvalidEventType
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "video_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{column: gorm.Expr(column + " + 1")}),
	}).Create(&seed).Error
}

// GetMetricsByVideoID 读取 video_metrics；不存在则返回零值指标（非错误）。
func (r *EventRepository) GetMetricsByVideoID(ctx context.Context, videoID uint) (VideoMetrics, error) {
	if videoID == 0 {
		return VideoMetrics{}, ErrInvalidArgument
	}
	var m VideoMetrics
	err := r.db.WithContext(ctx).Where("video_id = ?", videoID).First(&m).Error
	if err == nil {
		return m, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 尚无聚合行：与 Track 从未成功 Increment 等价，对外展示 0
		return VideoMetrics{VideoID: videoID}, nil
	}
	return VideoMetrics{}, err
}
