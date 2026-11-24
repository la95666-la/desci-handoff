// Package middleware 提供通用 gin 中间件（C 端/公共侧）。
package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"joincare_server/pkg/core/db"
)

// RateLimit 按"端点名 + 客户端 IP"做 Redis 固定窗口限流：
// window 内超过 limit 次返回 429。Redis 异常时 Incr 返回 0，天然 fail-open，
// 不会因限流组件故障拖垮业务（日志由 redis hook 记录）。
//
// 用法：loginV1.POST("/login", middleware.RateLimit("login", 20, time.Minute), login.Login)
func RateLimit(name string, limit int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := fmt.Sprintf("RATE:%s:%s", name, c.ClientIP())
		n := db.Redis().Incr(ctx, key)
		if n == 1 {
			db.Redis().Expire(ctx, key, window)
		}
		if n > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"status":  false,
				"code":    "RATE_LIMITED",
				"message": "too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}
