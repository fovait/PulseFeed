package event

import "time"

type EventType string

const (
	EventTypeImpression   EventType = "impression"
	EventTypeView         EventType = "view"
	EventTypePlayComplete EventType = "play_complete"
	EventTypeShare        EventType = "share"
)

// IsValid 判断事件类型是否为受支持的枚举值。
func (t EventType) IsValid() bool {
	switch t {
	case EventTypeImpression, EventTypeView, EventTypePlayComplete, EventTypeShare:
		return true
	default:
		return false
	}
}

type UserEvent struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	AccountID      uint      `gorm:"not null;index:idx_user_events_account_time,priority:1" json:"account_id"`
	VideoID        uint      `gorm:"not null;index:idx_user_events_video_time,priority:1" json:"video_id"`
	Type           EventType `gorm:"type:varchar(32);not null;index" json:"type"`
	IdempotencyKey string    `gorm:"type:varchar(128);not null;uniqueIndex" json:"idempotency_key"`
	OccurredAt     time.Time `gorm:"not null;index:idx_user_events_account_time,priority:2;index:idx_user_events_video_time,priority:2" json:"occurred_at"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type VideoMetrics struct {
	VideoID           uint      `gorm:"primaryKey" json:"video_id"`
	ImpressionCount   int64     `gorm:"not null;default:0" json:"impression_count"`
	ViewCount         int64     `gorm:"not null;default:0" json:"view_count"`
	PlayCompleteCount int64     `gorm:"not null;default:0" json:"play_complete_count"`
	ShareCount        int64     `gorm:"not null;default:0" json:"share_count"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (VideoMetrics) TableName() string {
	return "video_metrics"
}

type TrackRequest struct {
	VideoID        uint      `json:"video_id" binding:"required,min=1"`
	Type           EventType `json:"type" binding:"required"`
	IdempotencyKey string    `json:"idempotency_key" binding:"required,max=128"`
}

type TrackResponse struct {
	Event *UserEvent `json:"event"`
}

// GetVideoMetricsRequest 查询某条视频的聚合行为指标。
type GetVideoMetricsRequest struct {
	VideoID uint `json:"video_id" binding:"required,min=1"`
}

// GetVideoMetricsResponse 无埋点记录时各计数为 0。
type GetVideoMetricsResponse struct {
	Metrics VideoMetrics `json:"metrics"`
}
