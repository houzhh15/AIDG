package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/houzhh15/AIDG/pkg/logger"
)

// RequestLogger 写入结构化请求日志并注入 request_id
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID := uuid.NewString()
		c.Set("request_id", reqID)
		c.Writer.Header().Set("X-Request-ID", reqID)

		c.Next()

		duration := time.Since(start)

		logger.L().Info("http_request",
			"rid", reqID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
