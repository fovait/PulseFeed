package message

import "time"

type Message struct {
	ID uint `gorm:"primaryKey" json:"id"`

	FromID uint `gorm:"index:idx_messages_from_to,priority:1;index:idx_messages_to_from,priority:2;not null" json:"from_id"`
	ToID   uint `gorm:"index:idx_messages_from_to,priority:2;index:idx_messages_to_from,priority:1;not null" json:"to_id"`

	Content string `gorm:"type:text;not null" json:"content"`
	IsRead  bool   `gorm:"default:false" json:"is_read"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type SendRequest struct {
	ToID    uint   `json:"to_id" binding:"required,min=1"`
	Content string `json:"content" binding:"required,min=1,max=1000"`
}

type ListRequest struct {
	PeerID uint `json:"peer_id" binding:"required,min=1"`

	// 每页数量，不传默认 20，最大 50。
	Limit int `json:"limit" binding:"omitempty,min=1,max=50"`

	// 分页游标。
	// 第一页传 0。
	// 下一页传接口返回的 next_before_id。
	BeforeID uint `json:"before_id"`
}

type ListResponse struct {
	Messages     []Message `json:"messages"`
	NextBeforeID uint      `json:"next_before_id,omitempty"`
	HasMore      bool      `json:"has_more"`
}
