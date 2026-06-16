package video

import (
	"PulseFeed/internal/app"
	"errors"
	"net/http"
)

// 点赞相关的业务哨兵错误。handler 用 classifyLikeError 映射到正确的 HTTP 状态码,
// 而不是把业务冲突(如重复点赞)一律当成 500(服务器故障),污染错误率与告警。
var (
	ErrLikeNil       = errors.New("like is nil")
	ErrInvalidLike   = errors.New("video_id and account_id are required")
	ErrVideoNotFound = errors.New("video not found")
	ErrAlreadyLiked  = errors.New("user has liked this video")
	ErrNotLiked      = errors.New("user has not liked this video")
)

// classifyLikeError 把点赞业务错误映射为 HTTP 状态码:
// 参数错误 400 / 视频不存在 404 / 重复点赞或未点赞(幂等冲突)409;
// 其余未知错误回退到通用分类(默认 500)。
func classifyLikeError(err error) int {
	switch {
	case errors.Is(err, ErrLikeNil), errors.Is(err, ErrInvalidLike):
		return http.StatusBadRequest
	case errors.Is(err, ErrVideoNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyLiked), errors.Is(err, ErrNotLiked):
		return http.StatusConflict
	default:
		return app.ClassifyHTTPStatus(err)
	}
}
