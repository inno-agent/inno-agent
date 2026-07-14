package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/inno-agent/identity/internal/middleware"
	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

func TestAdapt_CorrelationID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log := zap.NewNop()
	r := gin.New()
	r.Use(middleware.Adapt(logger.CorrelationID))
	r.Use(middleware.Adapt(logger.InjectLogger(log)))
	r.Use(middleware.Adapt(logger.RequestLogger()))
	r.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get(logger.Header); got == "" {
		t.Fatal("expected X-Correlation-ID response header")
	}
}
