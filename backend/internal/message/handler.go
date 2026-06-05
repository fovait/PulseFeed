package message

import (
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func currentAccountID(c *gin.Context) (uint, bool) {
	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return 0, false
	}
	return accountID, true
}

// Send 发送私信。
//
// 请求：
//
//	{
//	  "to_id": 2,
//	  "content": "你好"
//	}
func (h *Handler) Send(c *gin.Context) {
	fromID, ok := currentAccountID(c)
	if !ok {
		return
	}

	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	msg, err := h.service.Send(c.Request.Context(), fromID, req)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, msg)
}

// List 查询当前用户和某个用户的聊天记录。
//
// 请求：
//
//	{
//	  "peer_id": 2,
//	  "limit": 20,
//	  "before_id": 0
//	}
func (h *Handler) List(c *gin.Context) {
	userID, ok := currentAccountID(c)
	if !ok {
		return
	}

	var req ListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.List(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListConversations 查询当前用户的私信会话列表。
func (h *Handler) ListConversations(c *gin.Context) {
	userID, ok := currentAccountID(c)
	if !ok {
		return
	}

	var req ListConversationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.ListConversations(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
