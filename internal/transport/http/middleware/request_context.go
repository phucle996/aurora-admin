package middleware

import (
	appctxKey "admin/internal/key"
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if reqID == "" {
			reqID = strings.TrimSpace(c.GetHeader("X-Request-Id"))
		}
		if reqID == "" {
			reqID = uuid.NewString()
		}

		ctx := context.WithValue(c.Request.Context(), appctxKey.KeyRequestID, reqID)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-ID", reqID)

		c.Next()
	}
}
