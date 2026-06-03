package event

import (
	"PulseFeed/internal/middleware/rabbitmq"
	"PulseFeed/internal/video"
	"context"
	"errors"
	"log"
	"strings"
	"time"
)

type EventService struct {
	repo      *EventRepository
	videoRepo *video.VideoRepository
	eventMQ   *rabbitmq.EventMQ
}

func NewEventService(
	repo *EventRepository,
	videoRepo *video.VideoRepository,
	eventMQ *rabbitmq.EventMQ,
) *EventService {
	return &EventService{
		repo:      repo,
		videoRepo: videoRepo,
		eventMQ:   eventMQ,
	}
}

func (s *EventService) Track(ctx context.Context, accountID uint, req TrackRequest) (*UserEvent, error) {
	// --- 1. 基础校验 ---
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if accountID == 0 || req.VideoID == 0 || req.IdempotencyKey == "" {
		return nil, ErrInvalidArgument
	}
	if !req.Type.IsValid() {
		return nil, ErrInvalidEventType
	}
	if s.repo == nil {
		return nil, ErrInvalidArgument
	}

	// --- 2. 视频必须存在（与点赞/评论一致）---
	// videoRepo 可为 nil（测试或未注入时跳过）
	if s.videoRepo != nil {
		exists, err := s.videoRepo.IsExist(ctx, req.VideoID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrVideoNotFound
		}
	}

	// --- 3. 组装待写入事件（OccurredAt 以服务端为准）---
	evt := &UserEvent{
		AccountID:      accountID,
		VideoID:        req.VideoID,
		Type:           req.Type,
		IdempotencyKey: req.IdempotencyKey,
		OccurredAt:     time.Now().UTC(),
	}

	// --- 4. 写明细；唯一键冲突 = 幂等重放 ---
	if err := s.repo.Save(ctx, evt); err != nil {
		if errors.Is(err, ErrDuplicate) {
			// 从库中取出「第一次成功写入」的那条，保证响应里带真实 id
			if existing, findErr := s.repo.FindByIdempotencyKey(ctx, req.IdempotencyKey); findErr == nil {
				return existing, nil
			}
			// 极端竞态：冲突了但还查不到，仍视为成功，返回内存对象
			return evt, nil
		}
		return nil, err
	}

	// --- 5. 仅首次写入才聚合 + 发 MQ（幂等重放不会走到这里）---
	if err := s.repo.Increment(ctx, evt.VideoID, evt.Type); err != nil {
		log.Printf("event: increment metrics failed (video=%d type=%s): %v", evt.VideoID, evt.Type, err)
	}
	if s.eventMQ != nil {
		if err := s.eventMQ.Publish(ctx, string(evt.Type), evt.AccountID, evt.VideoID); err != nil {
			log.Printf("event: publish failed (video=%d type=%s): %v", evt.VideoID, evt.Type, err)
		}
	}

	return evt, nil
}

// GetVideoMetrics 返回视频行为汇总；可选校验视频存在。
func (s *EventService) GetVideoMetrics(ctx context.Context, videoID uint) (VideoMetrics, error) {
	if videoID == 0 {
		return VideoMetrics{}, ErrInvalidArgument
	}
	if s.repo == nil {
		return VideoMetrics{}, ErrInvalidArgument
	}

	if s.videoRepo != nil {
		exists, err := s.videoRepo.IsExist(ctx, videoID)
		if err != nil {
			return VideoMetrics{}, err
		}
		if !exists {
			return VideoMetrics{}, ErrVideoNotFound
		}
	}

	return s.repo.GetMetricsByVideoID(ctx, videoID)
}
