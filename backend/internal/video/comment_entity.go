package video

import "time"

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"not null" json:"username"`
	VideoID   uint      `gorm:"index" json:"video_id" binding:"required,min=1"`
	AuthorID  uint      `gorm:"index" json:"author_id"`
	Content   string    `gorm:"type:text" json:"content" binding:"required,min=1,max=500"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type PublishCommentRequest struct {
	VideoID uint   `json:"video_id"`
	Content string `json:"content"`
}

type DeleteCommentRequest struct {
	CommentID uint `json:"comment_id"`
}

type GetAllCommentsRequest struct {
	VideoID uint `json:"video_id"`
}
