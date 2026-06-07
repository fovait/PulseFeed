package worker

import (
	"PulseFeed/internal/event"
	"PulseFeed/internal/middleware/rabbitmq"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/video"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// EventWorker 消费 user.events.metrics 队列中的用户行为事件。
// API 已写入 user_events / video_metrics，这里只负责衍生逻辑（如 Redis 热榜）。
type EventWorker struct {
	ch    *amqp.Channel
	cache *rediscache.Client
	queue string
}

func NewEventWorker(ch *amqp.Channel, cache *rediscache.Client, queue string) *EventWorker {
	return &EventWorker{ch: ch, cache: cache, queue: queue}
}

func (w *EventWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.cache == nil {
		return errors.New("event worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}

	if err := w.ch.Qos(10, 0, false); err != nil {
		return err
	}

	deliveries, err := w.ch.Consume(
		w.queue,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			w.handleDelivery(ctx, d)
		}
	}
}

func (w *EventWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			_ = d.Nack(false, true)
			return
		default:
		}
		if err := w.process(ctx, d.Body); err != nil {
			if i >= maxRetries {
				log.Printf("event worker: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)
				_ = d.Nack(false, false)
				return
			}
			wait := time.Duration(1<<uint(i)) * time.Second
			log.Printf("event worker: 处理失败, %v 后重试 (%d/%d): %v", wait, i+1, maxRetries, err)
			sleepOrDone(ctx, wait)
			continue
		}
		_ = d.Ack(false)
		return
	}
}

func (w *EventWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq.UserEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		log.Printf("event worker: invalid json: %v", err)
		return nil // 毒消息，Ack 丢弃
	}
	if evt.AccountID == 0 || evt.VideoID == 0 {
		log.Printf("event worker: invalid event: %+v", evt)
		return nil
	}

	delta := popularityDeltaForEventType(evt.Type)
	if delta == 0 {
		log.Printf("event worker: unknown type, skip: %s", evt.Type)
		return nil
	}

	return video.UpdatePopularityCache(ctx, w.cache, evt.VideoID, delta)
}

// popularityDeltaForEventType 行为对「分钟热榜」的权重（可按产品调参）。
func popularityDeltaForEventType(t string) int64 {
	switch event.EventType(t) {
	case event.EventTypeImpression:
		return 1
	case event.EventTypeView:
		return 2
	case event.EventTypePlayComplete:
		return 5
	case event.EventTypeShare:
		return 10
	default:
		return 0
	}
}
