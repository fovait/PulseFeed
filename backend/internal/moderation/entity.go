package moderation

import "time"

type ContentType string
type AuditStatus string

const (
	ContentTypeVideo   ContentType = "video"
	ContentTypeComment ContentType = "comment"

	AuditStatusPending  AuditStatus = "pending"
	AuditStatusApproved AuditStatus = "approved"
	AuditStatusRejected AuditStatus = "rejected"
	AuditStatusHidden   AuditStatus = "hidden"
)

// Valid 判断内容类型是否受支持。
func (t ContentType) Valid() bool {
	switch t {
	case ContentTypeVideo, ContentTypeComment:
		return true
	default:
		return false
	}
}

// IsReviewDecision 判断是否为合法的审核结论（不含 pending）。
func (s AuditStatus) IsReviewDecision() bool {
	switch s {
	case AuditStatusApproved, AuditStatusRejected, AuditStatusHidden:
		return true
	default:
		return false
	}
}

type ContentReport struct {
	ID         uint        `gorm:"primaryKey" json:"id"`
	ReporterID uint        `gorm:"not null;index" json:"reporter_id"`
	ReviewerID uint        `gorm:"index" json:"reviewer_id,omitempty"`
	TargetID   uint        `gorm:"not null;index:idx_content_reports_target,priority:2" json:"target_id"`
	TargetType ContentType `gorm:"type:varchar(32);not null;index:idx_content_reports_target,priority:1" json:"target_type"`
	Reason     string      `gorm:"type:varchar(255);not null" json:"reason"`
	Status     AuditStatus `gorm:"type:varchar(32);not null;default:pending;index" json:"status"`
	ReviewNote string      `gorm:"type:varchar(255)" json:"review_note,omitempty"`
	CreatedAt  time.Time   `gorm:"autoCreateTime" json:"created_at"`
	ReviewedAt *time.Time  `json:"reviewed_at,omitempty"`
}

type ReportRequest struct {
	TargetID   uint        `json:"target_id" binding:"required,min=1"`
	TargetType ContentType `json:"target_type" binding:"required"`
	Reason     string      `json:"reason" binding:"required,min=1,max=255"`
}

type ReviewRequest struct {
	ReportID uint        `json:"report_id" binding:"required,min=1"`
	Status   AuditStatus `json:"status" binding:"required"`
	Note     string      `json:"note" binding:"omitempty,max=255"`
}

type ReportResponse struct {
	Report *ContentReport `json:"report"`
}

type ListReportsRequest struct {
	Status AuditStatus `json:"status"`
	Limit  int         `json:"limit" binding:"omitempty,min=1,max=200"`
}

type ListReportsResponse struct {
	Reports []ContentReport `json:"reports"`
}

type IsAdminResponse struct {
	IsAdmin bool `json:"is_admin"`
}
