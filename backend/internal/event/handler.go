package event

import (
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type EventHandler struct {
	service *EventService
}

func NewEventHandler(service *EventService) *EventHandler {
	return &EventHandler{service: service}
}

// Track 上报一次用户行为事件（需登录）。
func (h *EventHandler) Track(c *gin.Context) {
	if h == nil || h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event service is not initialized"})
		return
	}

	var req TrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.VideoID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video_id is required"})
		return
	}
	if req.IdempotencyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "idempotency_key is required"})
		return
	}

	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	evt, err := h.service.Track(c.Request.Context(), accountID, req)
	if err != nil {
		c.JSON(classifyEventError(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, TrackResponse{Event: evt})
}

// GetVideoMetrics 查询视频行为指标（登录与否可按产品决定；下面示例不强制登录）。
func (h *EventHandler) GetVideoMetrics(c *gin.Context) {
	var req GetVideoMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	metrics, err := h.service.GetVideoMetrics(c.Request.Context(), req.VideoID)
	if err != nil {
		c.JSON(classifyEventError(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetVideoMetricsResponse{Metrics: metrics})
}

// classifyEventError 把 event 包错误映射为 HTTP 状态码。
func classifyEventError(err error) int {
	switch {
	case errors.Is(err, ErrInvalidArgument),
		errors.Is(err, ErrInvalidEventType):
		return http.StatusBadRequest
	case errors.Is(err, ErrVideoNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
