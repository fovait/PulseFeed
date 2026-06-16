package ratelimit

import (
	"PulseFeed/internal/middleware/jwt"
	rediscache "PulseFeed/internal/middleware/redis"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type KeyFunc func(g *gin.Context) (string, bool)

func Limit(
	cache *rediscache.Client,
	keyPrefix string,
	maxRequest int64,
	window time.Duration,
	keyFunc KeyFunc,
) gin.HandlerFunc {
	// 压测开关：RATELIMIT_DISABLED=1 时全局旁路限流（仅用于本地压测，勿在生产开启）。
	// 在构造期判定一次即可，避免每个请求都读环境变量。
	if os.Getenv("RATELIMIT_DISABLED") == "1" {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if cache == nil || keyFunc == nil || maxRequest <= 0 || window <= 0 {
			c.Next()
			return
		}

		subject, ok := keyFunc(c)
		if !ok {
			c.Next()
			return
		}

		key := buildKey(keyPrefix, subject)
		count, err := cache.IncrementWithExpire(c.Request.Context(), key, window)
		if err != nil {
			c.Next()
			return
		}

		if count > maxRequest {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}

func buildKey(keyPrefix, subject string) string {
	keyPrefix = strings.TrimSpace(keyPrefix)
	if keyPrefix == "" {
		keyPrefix = "default"
	}
	return fmt.Sprintf("PulseFeed:ratelimit:%s:%s", keyPrefix, strings.TrimSpace(subject))
}

func KeyByIP(c *gin.Context) (string, bool) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "", false
	}
	return ip, true
}

func KeyByAccount(c *gin.Context) (string, bool) {
	accountID, err := jwt.GetAccountID(c)
	if err != nil || accountID == 0 {
		return "", false
	}
	return strconv.FormatUint(uint64(accountID), 10), true
}
