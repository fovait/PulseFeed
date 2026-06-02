package feed

import (
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type FeedHandler struct {
	service *FeedService
}

func NewFeedHandler(service *FeedService) *FeedHandler {
	return &FeedHandler{
		service: service,
	}
}

// optionalViewerAccountID 尝试从 JWT 中获取当前用户 ID。
// 如果用户未登录，返回 0。
//
// 适合这些接口：
// 1. 最新流
// 2. 热门流
// 3. 标签流
//
// 因为这些接口允许游客访问。
// viewerAccountID = 0 时，Service 层可以统一认为 is_liked=false。
func optionalViewerAccountID(c *gin.Context) uint {
	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		return 0
	}
	return accountID
}

// requireViewerAccountID 强制要求用户登录。
// 如果没有登录，直接返回 401。
//
// 适合关注流，因为关注流必须知道“我关注了谁”。
func requireViewerAccountID(c *gin.Context) (uint, bool) {
	accountID, err := jwt.GetAccountID(c)
	if err != nil || accountID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return 0, false
	}
	return accountID, true
}

// unixSecondsToTime 把秒级时间戳转换成 time.Time。
// beforeTime == 0 表示第一页，不设置游标时间。
func unixSecondsToTime(beforeTime int64) time.Time {
	if beforeTime <= 0 {
		return time.Time{}
	}
	return time.Unix(beforeTime, 0)
}

// ensureVideoListNotNil 保证返回给前端的是 []，而不是 null。
func ensureVideoListNotNil(items []FeedVideoItem) []FeedVideoItem {
	if items == nil {
		return []FeedVideoItem{}
	}
	return items
}

// ListLatest 获取全站最新视频流。
// 分页方式：按 create_time 倒序。
// 请求参数：
//
//	{
//	  "limit": 20,
//	  "before_time": 1717200000
//	}
//
// before_time = 0 表示第一页。
// 下一页使用响应里的 next_before_time。
func (h *FeedHandler) ListLatest(c *gin.Context) {
	var req ListLatestRequest

	// 1. 绑定 JSON 请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	// 2. 统一修正 limit
	limit := NormalizeLimit(req.Limit)

	// 3. 秒级时间戳转 time.Time
	beforeTime := unixSecondsToTime(req.BeforeTime)

	// 4. 最新流允许游客访问
	viewerAccountID := optionalViewerAccountID(c)

	// 5. 调用 Service
	resp, err := h.service.ListLatest(
		c.Request.Context(),
		limit,
		beforeTime,
		viewerAccountID,
	)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	// 6. 避免 video_list 返回 null
	resp.VideoList = ensureVideoListNotNil(resp.VideoList)

	c.JSON(http.StatusOK, resp)
}

// ListByFollowing 获取当前登录用户关注的人发布的视频。
// 分页方式：按 create_time 倒序。
// 注意：关注流必须登录。
func (h *FeedHandler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	limit := NormalizeLimit(req.Limit)
	beforeTime := unixSecondsToTime(req.BeforeTime)

	// 关注流必须登录，不能用 optionalViewerAccountID。
	viewerAccountID, ok := requireViewerAccountID(c)
	if !ok {
		return
	}

	resp, err := h.service.ListByFollowing(
		c.Request.Context(),
		limit,
		beforeTime,
		viewerAccountID,
	)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	resp.VideoList = ensureVideoListNotNil(resp.VideoList)

	c.JSON(http.StatusOK, resp)
}

// ListByLikes 按点赞数获取视频列表。
// 排序规则：likes_count DESC, id DESC。
//
// 第一页请求：
//
//	{
//	  "limit": 20
//	}
//
// 下一页请求：
//
//	{
//	  "limit": 20,
//	  "cursor": {
//	    "likes_count": 100,
//	    "id": 123
//	  }
//	}
func (h *FeedHandler) ListByLikes(c *gin.Context) {
	var req ListByLikesRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	limit := NormalizeLimit(req.Limit)

	// 点赞榜允许游客访问。
	viewerAccountID := optionalViewerAccountID(c)

	// 校验 cursor。
	// cursor == nil 表示第一页。
	if req.Cursor != nil {
		if req.Cursor.LikesCount < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cursor.likes_count must be >= 0"})
			return
		}
		if req.Cursor.ID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cursor.id must be > 0"})
			return
		}
	}

	resp, err := h.service.ListByLikes(
		c.Request.Context(),
		limit,
		req.Cursor,
		viewerAccountID,
	)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	resp.VideoList = ensureVideoListNotNil(resp.VideoList)

	c.JSON(http.StatusOK, resp)
}

// ListByPopularity 获取热门视频流。
// 推荐使用 Redis 热榜分页。
// Cursor 含义：
//
//	as_of：固定榜单快照时间，通常是分钟级 Unix 时间戳
//	offset：下一页从排行榜第几个位置开始
//
// 第一页请求：
//
//	{
//	  "limit": 20
//	}
//
// 下一页请求：
//
//	{
//	  "limit": 20,
//	  "cursor": {
//	    "as_of": 1717200000,
//	    "offset": 20
//	  }
//	}
func (h *FeedHandler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	limit := NormalizeLimit(req.Limit)

	// 热门流允许游客访问。
	viewerAccountID := optionalViewerAccountID(c)

	// cursor == nil 表示第一页。
	// 第一页由 Service 自动选择当前分钟作为 as_of。
	if req.Cursor != nil {
		if req.Cursor.AsOf < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cursor.as_of must be >= 0"})
			return
		}
		if req.Cursor.Offset < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cursor.offset must be >= 0"})
			return
		}
	}

	resp, err := h.service.ListByPopularity(
		c.Request.Context(),
		limit,
		req.Cursor,
		viewerAccountID,
	)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	resp.VideoList = ensureVideoListNotNil(resp.VideoList)

	c.JSON(http.StatusOK, resp)
}

// ListByTag 根据标签名获取视频列表。
// 例如请求：
//
//	{
//	  "tag_name": "Go",
//	  "limit": 20
//	}
func (h *FeedHandler) ListByTag(c *gin.Context) {
	var req ListByTagRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	// 标签名去掉前后空格，避免 "   " 这种无效输入。
	tagName := strings.TrimSpace(req.TagName)
	if tagName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_name is required"})
		return
	}

	limit := NormalizeLimit(req.Limit)
	beforeTime := unixSecondsToTime(req.BeforeTime)

	// 标签流允许游客访问。
	viewerAccountID := optionalViewerAccountID(c)

	resp, err := h.service.ListByTag(
		c.Request.Context(),
		tagName,
		limit,
		beforeTime,
		viewerAccountID,
	)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	resp.VideoList = ensureVideoListNotNil(resp.VideoList)

	c.JSON(http.StatusOK, resp)
}
