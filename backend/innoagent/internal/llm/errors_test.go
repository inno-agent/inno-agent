package llm

import (
	"errors"
	"fmt"
	"testing"
)

func TestProviderError_Error(t *testing.T) {
	err := newProviderError(400, "bad request")
	msg := err.Error()

	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestProviderError_StatusCode(t *testing.T) {
	err := newProviderError(503, "service unavailable")
	if err.StatusCode != 503 {
		t.Fatalf("expected 503, got %d", err.StatusCode)
	}
	if err.Message != "service unavailable" {
		t.Fatalf("expected 'service unavailable', got %q", err.Message)
	}
}

func TestProviderError_As(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", newProviderError(400, "bad"))

	var pe *ProviderError
	if !errors.As(wrapped, &pe) {
		t.Fatal("expected errors.As to find ProviderError")
	}
	if pe.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", pe.StatusCode)
	}
}

func TestErrEmptyResponse(t *testing.T) {
	if ErrEmptyResponse.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestErrEmptyMessage(t *testing.T) {
	if ErrEmptyMessage.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
}
