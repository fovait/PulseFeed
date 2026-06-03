package video

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) CreateComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *CommentRepository) DeleteByID(ctx context.Context, id uint) (deleted bool, err error) {
	if id == 0 {
		return false, nil
	}

	res := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&Comment{})

	return res.RowsAffected > 0, res.Error
}

func (r *CommentRepository) GetAllComments(ctx context.Context, videoID uint) ([]Comment, error) {
	var comments []Comment
	err := r.db.WithContext(ctx).
		Where("video_id = ?", videoID).
		Order("created_at DESC").
		Limit(200).
		Find(&comments).Error
	return comments, err
}

func (r *CommentRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id uint) (*Comment, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepository) ApplyPublishTx(ctx context.Context, c *Comment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id").First(&Video{}, c.VideoID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		if err := tx.Create(c).Error; err != nil {
			return err
		}

		return tx.Model(&Video{}).
			Where("id = ?", c.VideoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
	})
}
