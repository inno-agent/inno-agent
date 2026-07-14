package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
