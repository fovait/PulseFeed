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

func (r *Repository) ListConversations(ctx context.Context, userID uint, limit int) ([]Conversation, error) {
	if userID == 0 {
		return []Conversation{}, nil
	}
	if limit <= 0 {
		limit = 50
	}

	var latestMessages []Message
	if err := r.db.WithContext(ctx).Raw(`
		SELECT m.*
		FROM messages m
		JOIN (
			SELECT
				CASE WHEN from_id = ? THEN to_id ELSE from_id END AS peer_id,
				MAX(id) AS max_id
			FROM messages
			WHERE from_id = ? OR to_id = ?
			GROUP BY peer_id
		) latest ON latest.max_id = m.id
		ORDER BY m.id DESC
		LIMIT ?
	`, userID, userID, userID, limit).Scan(&latestMessages).Error; err != nil {
		return nil, err
	}

	type unreadRow struct {
		PeerID      uint  `gorm:"column:peer_id"`
		UnreadCount int64 `gorm:"column:unread_count"`
	}
	var unreadRows []unreadRow
	if err := r.db.WithContext(ctx).
		Model(&Message{}).
		Select("from_id AS peer_id, COUNT(*) AS unread_count").
		Where("to_id = ? AND is_read = ?", userID, false).
		Group("from_id").
		Scan(&unreadRows).Error; err != nil {
		return nil, err
	}

	unreadByPeer := make(map[uint]int64, len(unreadRows))
	for _, row := range unreadRows {
		unreadByPeer[row.PeerID] = row.UnreadCount
	}

	conversations := make([]Conversation, 0, len(latestMessages))
	for _, msg := range latestMessages {
		peerID := msg.FromID
		if msg.FromID == userID {
			peerID = msg.ToID
		}
		conversations = append(conversations, Conversation{
			PeerID:      peerID,
			LastMessage: msg,
			UnreadCount: unreadByPeer[peerID],
			UpdatedAt:   msg.CreatedAt,
		})
	}

	return conversations, nil
}
