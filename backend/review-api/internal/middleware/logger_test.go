package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestLogger_InjectsIntoContext(t *testing.T) {
	var gotLogger *zap.Logger
	handler := Logger(zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLogger = LoggerFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotLogger == nil {
		t.Fatal("expected logger to be injected")
	}
}

func TestLoggerFromContext_ReturnsNopIfAbsent(t *testing.T) {
	log := LoggerFromContext(context.Background())
	if log == nil {
		t.Fatal("expected nop logger, got nil")
	}
}

func TestWithLogger(t *testing.T) {
	logger := zap.NewNop()
	ctx := WithLogger(context.Background(), logger)
	got := LoggerFromContext(ctx)
	if got != logger {
		t.Fatal("expected same logger instance")
	}
}
