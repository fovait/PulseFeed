package video

import (
	"PulseFeed/internal/app"
	"PulseFeed/internal/middleware/jwt"

	"github.com/gin-gonic/gin"
)

type LikeHandler struct {
	service *LikeService
}

func NewLikeHandler(service *LikeService) *LikeHandler {
	return &LikeHandler{service: service}
}

func (lh *LikeHandler) Like(c *gin.Context) {
	req, accountID, ok := bindLikeRequestAndAccount(c)
	if !ok {
		return
	}

	like := &Like{
		VideoID:   req.VideoID,
		AccountID: accountID,
	}

	if err := lh.service.Like(c.Request.Context(), like); err != nil {
		c.JSON(classifyLikeError(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "like success"})
}

func (lh *LikeHandler) UnLike(c *gin.Context) {
	req, accountID, ok := bindLikeRequestAndAccount(c)
	if !ok {
		return
	}

	like := &Like{
		VideoID:   req.VideoID,
		AccountID: accountID,
	}

	if err := lh.service.UnLike(c.Request.Context(), like); err != nil {
		c.JSON(classifyLikeError(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "unlike success"})
}

func (lh *LikeHandler) IsLiked(c *gin.Context) {
	req, accountID, ok := bindLikeRequestAndAccount(c)
	if !ok {
		return
	}

	isLiked, err := lh.service.IsLiked(c.Request.Context(), req.VideoID, accountID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"is_liked": isLiked})
}

func (lh *LikeHandler) ListMyLikedVideos(c *gin.Context) {
	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}

	videos, err := lh.service.ListLikedVideos(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return
	}
	if videos == nil {
		videos = []Video{}
	}
	c.JSON(200, videos)
}

func bindLikeRequestAndAccount(c *gin.Context) (LikeRequest, uint, bool) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return LikeRequest{}, 0, false
	}

	if req.VideoID == 0 {
		c.JSON(400, gin.H{"error": "video_id is required"})
		return LikeRequest{}, 0, false
	}

	accountID, err := jwt.GetAccountID(c)
	if err != nil {
		c.JSON(app.ClassifyHTTPStatus(err), gin.H{"error": err.Error()})
		return LikeRequest{}, 0, false
	}

	return req, accountID, true
}
