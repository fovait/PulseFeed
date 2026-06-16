package video

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestClassifyLikeError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"invalid-request", ErrInvalidLike, http.StatusBadRequest},
		{"like-nil", ErrLikeNil, http.StatusBadRequest},
		{"video-not-found", ErrVideoNotFound, http.StatusNotFound},
		{"already-liked", ErrAlreadyLiked, http.StatusConflict},
		{"not-liked", ErrNotLiked, http.StatusConflict},
		// 包裹后仍应正确分类(service 的 fallback 事务里会被 GORM 等包一层)
		{"wrapped-already-liked", fmt.Errorf("tx failed: %w", ErrAlreadyLiked), http.StatusConflict},
		// 未知错误兜底 500
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if got := classifyLikeError(tc.err); got != tc.want {
			t.Errorf("%s: classifyLikeError = %d, want %d", tc.name, got, tc.want)
		}
	}
}
