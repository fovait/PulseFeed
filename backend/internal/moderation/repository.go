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
	ListReports(ctx context.Context, status AuditStatus, limit int) ([]ContentReport, error)
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

// ListReports 列举举报记录，可按状态过滤；status 为空时返回全部。
func (r *Repository) ListReports(ctx context.Context, status AuditStatus, limit int) ([]ContentReport, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var reports []ContentReport
	q := r.db.WithContext(ctx).Model(&ContentReport{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("created_at DESC").Limit(limit).Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
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

// LatestStatuses 批量返回多个目标的最新审核状态，替代逐条调用 LatestStatus 的 N+1 模式。
// 无记录的 targetID 不会出现在返回的 map 中（调用方视为"未被举报/可见"）。
func (r *Repository) LatestStatuses(ctx context.Context, targetType ContentType, targetIDs []uint) (map[uint]AuditStatus, error) {
	if len(targetIDs) == 0 {
		return map[uint]AuditStatus{}, nil
	}
	var reports []ContentReport
	err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id IN ?", targetType, targetIDs).
		Order("created_at DESC").
		Find(&reports).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uint]AuditStatus, len(targetIDs))
	for _, rep := range reports {
		if _, seen := result[rep.TargetID]; !seen {
			// DESC 排序后第一次出现 = 该 targetID 最新的一条记录
			result[rep.TargetID] = rep.Status
		}
	}
	return result, nil
}