package telemetry

import (
	"time"

	"github.com/gin-gonic/gin"
)

// GinMiddleware records HTTP metrics for gin routers.
func GinMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		trackInFlight(serviceName, 1)
		defer trackInFlight(serviceName, -1)

		c.Next()
		observe(serviceName, c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}
