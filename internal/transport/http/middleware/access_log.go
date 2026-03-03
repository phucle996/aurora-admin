package middleware

import (
	"admin/pkg/logger"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		userAgent := strings.TrimSpace(c.Request.UserAgent())
		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		ctx := c.Request.Context()
		logger.AccessLog(
			ctx,
			"method=%s path=%s status=%d latency=%s ip=%s ua=%q",
			method,
			path,
			status,
			latency,
			clientIP,
			userAgent,
		)
	}
}
