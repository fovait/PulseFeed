package moderation

import (
	"PulseFeed/internal/middleware/jwt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireAdmin 仅允许管理员访问（须挂在 JWTAuth 之后）。
func RequireAdmin(checker AdminChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, err := jwt.GetAccountID(c)
		if err != nil || accountID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
			return
		}
		if checker == nil || !checker.IsAdmin(accountID) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin permission required"})
			return
		}
		c.Next()
	}
}
