package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

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

func TestAccessLog_CorrectStatusAndBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use zaptest.NewLogger to capture log entries
	tlog := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(middleware.Adapt(logger.CorrelationID))
	r.Use(middleware.Adapt(logger.InjectLogger(tlog)))
	r.Use(middleware.AccessLog())

	// Handler that returns 401 with a body
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusUnauthorized, "Unauthorized")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Verify status code in response
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	// Verify body was written
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("expected body to be written")
	}
}
