package moderation

import (
	"context"
	"strings"
)

type ModerationService struct {
	repo  ModerationRepository
	admin AdminChecker
}

func NewModerationService(repo ModerationRepository, admin AdminChecker) *ModerationService {
	return &ModerationService{repo: repo, admin: admin}
}

// Report 创建一条内容举报，初始状态为 pending。
func (s *ModerationService) Report(ctx context.Context, reporterID uint, req ReportRequest) (*ContentReport, error) {
	if reporterID == 0 || req.TargetID == 0 || strings.TrimSpace(req.Reason) == "" {
		return nil, ErrInvalidArgument
	}
	if !req.TargetType.Valid() {
		return nil, ErrInvalidTargetType
	}
	if s.repo == nil {
		return nil, ErrInvalidArgument
	}

	report := &ContentReport{
		ReporterID: reporterID,
		TargetID:   req.TargetID,
		TargetType: req.TargetType,
		Reason:     strings.TrimSpace(req.Reason),
		Status:     AuditStatusPending,
	}
	if err := s.repo.CreateReport(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
}

// ListReports 列举举报记录（仅管理员可调用）。status 为空表示全部。
func (s *ModerationService) ListReports(ctx context.Context, reviewerID uint, status AuditStatus, limit int) ([]ContentReport, error) {
	if reviewerID == 0 {
		return nil, ErrInvalidArgument
	}
	if s.repo == nil {
		return nil, ErrInvalidArgument
	}
	if s.admin == nil || !s.admin.IsAdmin(reviewerID) {
		return nil, ErrForbidden
	}
	if status != "" && status != AuditStatusPending && !status.IsReviewDecision() {
		return nil, ErrInvalidStatus
	}
	return s.repo.ListReports(ctx, status, limit)
}

// IsAdmin 直接暴露给上层判断当前用户是否管理员。
func (s *ModerationService) IsAdmin(accountID uint) bool {
	return s != nil && s.admin != nil && s.admin.IsAdmin(accountID)
}

// Review 对一条举报做出审核结论（仅管理员可调用）。
func (s *ModerationService) Review(ctx context.Context, reviewerID uint, req ReviewRequest) error {
	if reviewerID == 0 || req.ReportID == 0 {
		return ErrInvalidArgument
	}
	if !req.Status.IsReviewDecision() {
		return ErrInvalidStatus
	}
	if s.repo == nil {
		return ErrInvalidArgument
	}
	if s.admin == nil || !s.admin.IsAdmin(reviewerID) {
		return ErrForbidden
	}
	return s.repo.UpdateReview(ctx, req.ReportID, reviewerID, req.Status, strings.TrimSpace(req.Note))
}

// IsVisible 根据目标最近一次举报记录的审核结论判断内容是否可见。
//
// 规则（见 content_reports 最新一条）：
//   - 无记录 / pending / approved → 可见（待审内容仍会展示，若要先审后发需在发布链路另做）
//   - rejected / hidden → 不可见
//
// 推荐流通过 VideoVisibilityChecker 只检查 ContentTypeVideo。
func (s *ModerationService) IsVisible(ctx context.Context, targetType ContentType, targetID uint) (bool, error) {
	if s.repo == nil {
		return true, nil
	}
	if targetID == 0 || !targetType.Valid() {
		return true, nil
	}
	status, found, err := s.repo.LatestStatus(ctx, targetType, targetID)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	switch status {
	case AuditStatusHidden, AuditStatusRejected:
		return false, nil
	default:
		return true, nil
	}
}
