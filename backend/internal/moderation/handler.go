package moderation

import (
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ModerationHandler 处理举报与审核 HTTP 接口。
type ModerationHandler struct {
	service *ModerationService
}

func NewModerationHandler(service *ModerationService) *ModerationHandler {
	return &ModerationHandler{service: service}
}

// Report 用户举报视频或评论（需登录）。
//
// 请求示例：
//
//	{
//	  "target_type": "video",
//	  "target_id": 1,
//	  "reason": "涉嫌违规"
//	}
func (h *ModerationHandler) Report(c *gin.Context) {
	if h == nil || h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "moderation service is not initialized"})
		return
	}

	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		return
	}

	reporterID, err := jwt.GetAccountID(c)
	if err != nil || reporterID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	report, err := h.service.Report(c.Request.Context(), reporterID, req)
	if err != nil {
		c.JSON(classifyModerationError(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ReportResponse{Report: report})
}

// Review 管理员对举报做出结论（需登录且 account_id 在管理员白名单内）。
//
// 请求示例：
//
//	{
//	  "report_id": 1,
//	  "status": "hidden",
//	  "note": "违反社区规范"
//	}
//
// 路由层使用 RequireAdmin 中间件；Service.Review 内会再次校验，防止绕过。
func (h *ModerationHandler) Review(c *gin.Context) {
	if h == nil || h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "moderation service is not initialized"})
		return
	}

	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if req.ReportID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "report_id is required"})
		return
	}

	reviewerID, err := jwt.GetAccountID(c)
	if err != nil || reviewerID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	if err := h.service.Review(c.Request.Context(), reviewerID, req); err != nil {
		c.JSON(classifyModerationError(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "review recorded"})
}

func classifyModerationError(err error) int {
	switch {
	case errors.Is(err, ErrInvalidArgument),
		errors.Is(err, ErrInvalidTargetType),
		errors.Is(err, ErrInvalidStatus):
		return http.StatusBadRequest
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
