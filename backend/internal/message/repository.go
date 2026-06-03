package message

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) AutoMigrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&Message{})
}

func (r *Repository) Create(ctx context.Context, msg *Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}

	msg.Content = strings.TrimSpace(msg.Content)

	if msg.FromID == 0 || msg.ToID == 0 {
		return errors.New("from_id and to_id are required")
	}
	if msg.Content == "" {
		return errors.New("content is required")
	}

	return r.db.WithContext(ctx).Create(msg).Error
}

func (r *Repository) ListConversation(
	ctx context.Context,
	userID uint,
	peerID uint,
	limit int,
	beforeID uint,
) ([]Message, error) {
	var messages []Message

	if userID == 0 || peerID == 0 {
		return []Message{}, nil
	}

	query := r.db.WithContext(ctx).
		Model(&Message{}).
		Where(
			"(from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)",
			userID,
			peerID,
			peerID,
			userID,
		)

	// before_id 用于加载更早的消息。
	// 因为消息 ID 递增，所以 id < before_id 表示更旧的消息。
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}

	if limit <= 0 {
		limit = 20
	}

	err := query.
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error

	return messages, err
}

func (r *Repository) MarkReadFromPeer(
	ctx context.Context,
	userID uint,
	peerID uint,
) error {
	if userID == 0 || peerID == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&Message{}).
		Where("from_id = ? AND to_id = ? AND is_read = ?", peerID, userID, false).
		Update("is_read", true).Error
}
