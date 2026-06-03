package worker

import (
	"PulseFeed/internal/middleware/rabbitmq"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SocialWorker struct {
	ch    *amqp.Channel
	queue string
}

func NewSocialWorker(ch *amqp.Channel, queue string) *SocialWorker {
	return &SocialWorker{ch: ch, queue: queue}
}

func (w *SocialWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil {
		return errors.New("social worker is not initialized")
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
		false,
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

func (w *SocialWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
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
				log.Printf("social worker: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)
				_ = d.Ack(false)
				return
			}
			wait := time.Duration(1<<uint(i)) * time.Second
			log.Printf("social worker: 处理失败, %v 后重试 (%d/%d): %v", wait, i+1, maxRetries, err)
			time.Sleep(wait)
			continue
		}
		_ = d.Ack(false)
		return
	}
}

func (w *SocialWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq.SocialEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		log.Printf("social worker: invalid json: %v", err)
		// 解析事件失败，直接丢弃
		return nil
	}
	if evt.FollowerID == 0 || evt.VloggerID == 0 {
		log.Printf("social worker: invalid event: %+v", evt)
		return nil
	}

	switch evt.Action {
	case "follow":
		return w.handleFollow(ctx, evt)
	case "unfollow":
		return w.handleUnfollow(ctx, evt)
	default:
		log.Printf("social worker: unknown action %s", evt.Action)
		return nil
	}
}

func (w *SocialWorker) handleFollow(ctx context.Context, evt rabbitmq.SocialEvent) error {
	// 注意：这里不再写 socials 表。
	// 关注关系已经在 SocialService.Follow 中同步写入 MySQL。
	//
	// 这里以后可以做：
	// 1. 给被关注者发通知
	// 2. 写行为日志
	// 3. 更新推荐画像
	// 4. 推送消息

	log.Printf(
		"social notify: user %d followed user %d, event_id=%s",
		evt.FollowerID,
		evt.VloggerID,
		evt.EventID,
	)

	return nil
}

func (w *SocialWorker) handleUnfollow(ctx context.Context, evt rabbitmq.SocialEvent) error {
	// 取消关注通常不需要通知对方。
	// 这里可以只记录行为日志，或者给推荐系统使用。

	log.Printf(
		"social log: user %d unfollowed user %d, event_id=%s",
		evt.FollowerID,
		evt.VloggerID,
		evt.EventID,
	)

	return nil
}
