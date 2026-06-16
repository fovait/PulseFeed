package rabbitmq

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type CommentMQ struct {
	ch *amqp.Channel
}

const (
	commentExchange   = "comment.events"
	commentQueue      = "comment.events"
	commentBindingKey = "comment.*"

	commentPublishRK = "comment.publish"
	commentDeleteRK  = "comment.delete"
)

type CommentEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	CommentID  uint      `json:"comment_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	VideoID    uint      `json:"video_id,omitempty"`
	AuthorID   uint      `json:"author_id,omitempty"`
	Content    string    `json:"content,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewCommentMQ(base *RabbitMQ) (*CommentMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := DeclareTopic(ch, commentExchange, commentQueue, commentBindingKey); err != nil {
		ch.Close()
		return nil, err
	}
	return &CommentMQ{ch: ch}, nil
}

func (c *CommentMQ) Close() error {
	if c == nil || c.ch == nil {
		return nil
	}
	return c.ch.Close()
}

// Publish 发布评论事件。eventID 由调用方（service 层）在 MQ/降级分叉之前生成并传入，
// 使 MQ 路径与降级直写共享同一个 eventID：当 MQ 模糊失败（broker 已收到但 ACK 超时）
// 触发降级时，降级写入与 worker 消费写入会落到同一 eventID，被唯一索引去重，避免重复评论。
func (c *CommentMQ) Publish(ctx context.Context, eventID, username string, videoID, authorID uint, content string) error {
	if eventID == "" || username == "" || videoID == 0 || authorID == 0 || content == "" {
		return errors.New("eventID, username, videoID, authorID and content are required")
	}
	return c.publish(ctx, "publish", commentPublishRK, CommentEvent{
		EventID:  eventID,
		Username: username,
		VideoID:  videoID,
		AuthorID: authorID,
		Content:  content,
	})
}

func (c *CommentMQ) Delete(ctx context.Context, commentID uint) error {
	if commentID == 0 {
		return errors.New("commentID is required")
	}
	return c.publish(ctx, "delete", commentDeleteRK, CommentEvent{
		CommentID: commentID,
	})
}

func (c *CommentMQ) publish(ctx context.Context, action, routingKey string, evt CommentEvent) error {
	if c == nil || c.ch == nil {
		return errors.New("comment mq is not initialized")
	}
	// 调用方未提供 eventID 时（如 Delete 路径）才自动生成；
	// Publish 路径由 service 层传入，保持 MQ 与降级路径 eventID 一致。
	if evt.EventID == "" {
		id, err := newEventID(16)
		if err != nil {
			return err
		}
		evt.EventID = id
	}
	evt.Action = action
	evt.OccurredAt = time.Now().UTC()
	return PublishJSON(ctx, c.ch, commentExchange, routingKey, evt)
}
