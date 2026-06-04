package moderation

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type ModerationRepository interface {
	CreateReport(ctx context.Context, r *ContentReport) error
	UpdateReview(ctx context.Context, id, reviewerID uint, status AuditStatus, note string) error
	LatestStatus(ctx context.Context, targetType ContentType, targetID uint) (AuditStatus, bool, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// AutoMigrate 创建/更新 content_reports 表结构。
func (r *Repository) AutoMigrate(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("moderation repository is not initialized")
	}
	return r.db.WithContext(ctx).AutoMigrate(&ContentReport{})
}

func (r *Repository) CreateReport(ctx context.Context, report *ContentReport) error {
	if report == nil {
		return errors.New("report is nil")
	}
	return r.db.WithContext(ctx).Create(report).Error
}

func (r *Repository) UpdateReview(ctx context.Context, id, reviewerID uint, status AuditStatus, note string) error {
	now := time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&ContentReport{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"reviewer_id": reviewerID,
			"review_note": note,
			"reviewed_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// LatestStatus 返回某目标最近一条举报的审核状态。无记录时 found=false。
func (r *Repository) LatestStatus(ctx context.Context, targetType ContentType, targetID uint) (AuditStatus, bool, error) {
	var report ContentReport
	err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").
		First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return report.Status, true, nil
}
