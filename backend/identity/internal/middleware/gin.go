package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

// Adapt wraps stdlib logger/tracing middleware for gin.
func Adapt(mw func(http.Handler) http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	}
}

// AccessLog writes one structured access log line per HTTP request (gin-native).
// This middleware reads status and bytes from gin's ResponseWriter, not a wrapper.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		bytes := c.Writer.Size()

		logger.FromContext(c.Request.Context()).Info(
			"http_request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", status),
			zap.Int("bytes", bytes),
			zap.Duration("duration", time.Since(start)),
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}
