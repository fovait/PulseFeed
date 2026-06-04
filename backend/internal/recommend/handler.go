package recommend

import (
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RecommendHandler 处理个性化推荐 HTTP 接口。
type RecommendHandler struct {
	service *RecommendService
}

func NewRecommendHandler(service *RecommendService) *RecommendHandler {
	return &RecommendHandler{service: service}
}

// Recommend 获取推荐流（需登录）。
//
// 请求示例：
//
//	{
//	  "limit": 20,
//	  "cursor": "",
//	  "debug": false
//	}
//
// 响应：
//
//	{
//	  "videos": [{ "video_id": 1, "score": 12.5, "source": "popularity", "reasons": ["latest","popularity"] }],
//	  "next_cursor": "v1:10.500000:1",
//	  "has_more": true
//	}
//
// 翻页：将上一页的 next_cursor 原样放入 cursor；曝光去重仍由服务端 FilterSeen 保证。
func (h *RecommendHandler) Recommend(c *gin.Context) {
	if h == nil || h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "recommend service is not initialized"})
		return
	}

	var req RecommendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	req.Cursor = strings.TrimSpace(req.Cursor)

	accountID, err := jwt.GetAccountID(c)
	if err != nil || accountID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	resp, err := h.service.Recommend(c.Request.Context(), accountID, req)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	// 避免 JSON 里 videos 为 null。
	if resp.Videos == nil {
		resp.Videos = []RankedVideo{}
	}

	c.JSON(http.StatusOK, resp)
}
