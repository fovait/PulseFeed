package rabbitmq

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type LikeMQ struct {
	ch *amqp.Channel
}

const (
	likeExchange   = "like.events"
	likeQueue      = "like.events"
	likeBindingKey = "like.*"

	likeLikeRK   = "like.like"
	likeUnlikeRK = "like.unlike"
)

type LikeEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	AccountID  uint      `json:"account_id"`
	VideoID    uint      `json:"video_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewLikeMQ(base *RabbitMQ) (*LikeMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := DeclareTopic(ch, likeExchange, likeQueue, likeBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	return &LikeMQ{ch: ch}, nil
}

func (l *LikeMQ) Close() error {
	if l == nil || l.ch == nil {
		return nil
	}
	return l.ch.Close()
}

func (l *LikeMQ) Like(ctx context.Context, AccountID, videoID uint) error {
	return l.publish(ctx, "like", likeLikeRK, AccountID, videoID)
}

func (l *LikeMQ) Unlike(ctx context.Context, AccountID, videoID uint) error {
	return l.publish(ctx, "unlike", likeUnlikeRK, AccountID, videoID)
}

func (l *LikeMQ) publish(ctx context.Context, action, routingKey string, AccountID, videoID uint) error {
	if l == nil || l.ch == nil {
		return errors.New("like mq is not initialized")
	}
	if AccountID == 0 || videoID == 0 {
		return errors.New("AccountID and videoID are required")
	}
	id, err := newEventID(16)
	if err != nil {
		return err
	}
	event := LikeEvent{
		EventID:    id,
		Action:     action,
		AccountID:  AccountID,
		VideoID:    videoID,
		OccurredAt: time.Now(),
	}
	return PublishJSON(ctx, l.ch, likeExchange, routingKey, event)
}
