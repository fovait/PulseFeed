package social

import (
	"PulseFeed/internal/account"
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type SocialHandler struct {
	service *SocialService
}

func NewSocialHandler(service *SocialService) *SocialHandler {
	return &SocialHandler{service: service}
}

// getCurrentAccountID 从 JWT 中获取当前登录用户 ID。
//
// 关注、取消关注、查询自己的关注/粉丝数据时，都需要当前登录用户。
func getCurrentAccountID(c *gin.Context) (uint, bool) {
	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return 0, false
	}
	return accountID, true
}

// bindJSONAllowEmpty 允许空 body。
//
// 为什么需要它？
// GetAllFollowers / GetAllVloggers 这类接口，有时候前端不传 body，
// 表示查询“我自己的粉丝/关注列表”。
// 如果直接 ShouldBindJSON，空 body 会报 EOF。
// 所以这里对 io.EOF 做兼容。
func bindJSONAllowEmpty(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return false
	}
	return true
}

// parseUintQuery 从 query 参数中解析 uint。
//
// 例如：
//
//	/social/followers?vlogger_id=10
func parseUintQuery(c *gin.Context, key string) (uint, bool, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, false, true
	}

	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " must be a positive integer"})
		return 0, true, false
	}

	return uint(n), true, true
}

// Follow 关注某个用户。
//
// 请求体：
//
//	{
//	  "vlogger_id": 20
//	}
//
// follower_id 不从前端传，而是从 JWT 中获取。
// 这样可以防止用户伪造别人的身份去关注。
func (h *SocialHandler) Follow(c *gin.Context) {
	var req FollowRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	if req.VloggerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vlogger_id is required"})
		return
	}

	followerID, ok := getCurrentAccountID(c)
	if !ok {
		return
	}

	social := &Social{
		FollowerID: followerID,
		VloggerID:  req.VloggerID,
	}

	if err := h.service.Follow(c.Request.Context(), social); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "followed"})
}

// Unfollow 取消关注某个用户。
//
// 请求体：
//
//	{
//	  "vlogger_id": 20
//	}
//
// follower_id 仍然从 JWT 获取。
func (h *SocialHandler) Unfollow(c *gin.Context) {
	var req UnfollowRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	if req.VloggerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vlogger_id is required"})
		return
	}

	followerID, ok := getCurrentAccountID(c)
	if !ok {
		return
	}

	social := &Social{
		FollowerID: followerID,
		VloggerID:  req.VloggerID,
	}

	if err := h.service.Unfollow(c.Request.Context(), social); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

func (h *SocialHandler) IsFollowed(c *gin.Context) {
	var req IsFollowedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if req.VloggerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vlogger_id is required"})
		return
	}
	followerID, ok := getCurrentAccountID(c)
	if !ok {
		return
	}
	isFollowed, err := h.service.IsFollowed(c.Request.Context(), &Social{
		FollowerID: followerID,
		VloggerID:  req.VloggerID,
	})
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, IsFollowedResponse{IsFollowed: isFollowed})
}

// GetAllFollowers 查询某个用户的粉丝列表。
//
// 支持两种调用方式：
//
//  1. 查询指定用户的粉丝：
//     GET /social/followers?vlogger_id=20
//
// 或 JSON：
//
//		{
//		  "vlogger_id": 20
//		}
//
//	 2. 不传 vlogger_id：
//	    默认查询当前登录用户自己的粉丝。
func (h *SocialHandler) GetAllFollowers(c *gin.Context) {
	vloggerID, found, ok := parseUintQuery(c, "vlogger_id")
	if !ok {
		return
	}

	if !found {
		var req GetAllFollowersRequest
		if !bindJSONAllowEmpty(c, &req) {
			return
		}
		vloggerID = req.VloggerID
	}

	// 如果没有指定 vlogger_id，就默认查询当前用户自己的粉丝。
	if vloggerID == 0 {
		accountID, ok := getCurrentAccountID(c)
		if !ok {
			return
		}
		vloggerID = accountID
	}

	followers, err := h.service.ListFollowers(c.Request.Context(), vloggerID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	if followers == nil {
		followers = []*account.Account{}
	}

	followerCount, err := h.service.CountFollowers(c.Request.Context(), vloggerID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetAllFollowersResponse{
		Followers:     followers,
		FollowerCount: followerCount,
	})
}

// GetAllVloggers 查询某个用户关注了哪些人。
//
// 支持两种调用方式：
//
//  1. 查询指定用户关注了谁：
//     GET /social/following?follower_id=10
//
// 或 JSON：
//
//		{
//		  "follower_id": 10
//		}
//
//	 2. 不传 follower_id：
//	    默认查询当前登录用户自己的关注列表。
func (h *SocialHandler) GetAllVloggers(c *gin.Context) {
	followerID, found, ok := parseUintQuery(c, "follower_id")
	if !ok {
		return
	}

	if !found {
		var req GetAllVloggersRequest
		if !bindJSONAllowEmpty(c, &req) {
			return
		}
		followerID = req.FollowerID
	}

	// 如果没有指定 follower_id，就默认查询当前用户自己的关注列表。
	if followerID == 0 {
		accountID, ok := getCurrentAccountID(c)
		if !ok {
			return
		}
		followerID = accountID
	}

	vloggers, err := h.service.ListFollowing(c.Request.Context(), followerID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	if vloggers == nil {
		vloggers = []*account.Account{}
	}

	followingCount, err := h.service.CountFollowing(c.Request.Context(), followerID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetAllVloggersResponse{
		Vloggers:     vloggers,
		VloggerCount: followingCount,
	})
}

// GetCounts 查询当前登录用户的粉丝数和关注数。
//
// 返回：
//
//	follower_count：有多少人关注我
//	vlogger_count：我关注了多少人
func (h *SocialHandler) GetCounts(c *gin.Context) {
	accountID, ok := getCurrentAccountID(c)
	if !ok {
		return
	}

	counts, err := h.service.GetSocialCounts(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, counts)
}
