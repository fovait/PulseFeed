package rabbitmq

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	EventExchange = "user.events"

	// 这个队列专门给 EventWorker 消费用。
	EventMetricsQueue = "user.events.metrics"

	// event.* 可以匹配：
	// event.impression
	// event.view
	// event.play_complete
	// event.share
	EventBindingKey = "event.*"
)

type UserEvent struct {
	EventID    string    `json:"event_id"`
	AccountID  uint      `json:"account_id"`
	VideoID    uint      `json:"video_id"`
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`
}

type EventMQ struct {
	ch *amqp.Channel
}

func NewEventMQ(base *RabbitMQ) (*EventMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := DeclareTopic(ch, EventExchange, EventMetricsQueue, EventBindingKey); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &EventMQ{ch: ch}, nil
}

func (p *EventMQ) Close() error {
	if p == nil || p.ch == nil {
		return nil
	}
	err := p.ch.Close()
	p.ch = nil
	return err
}

// Publish 发布事件到 RabbitMQ。
func (p *EventMQ) Publish(ctx context.Context, action string, AccountID, VideoID uint) error {
	if p == nil || p.ch == nil {
		return errors.New("event mq is not initialized")
	}
	if VideoID == 0 || AccountID == 0 {
		return errors.New("video_id, account_id is required")
	}

	id, err := newEventID(16)
	if err != nil {
		return err
	}

	event := &UserEvent{
		EventID:   id,
		AccountID: AccountID,
		VideoID:   VideoID,
		Type:      action,

		OccurredAt: time.Now().UTC(),
	}

	routingKey := "event." + action

	return PublishJSON(ctx, p.ch, EventExchange, routingKey, event)
}
