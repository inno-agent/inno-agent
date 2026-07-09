package tokensource

import (
	"context"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

func TestStatic_Token_ReturnsConfiguredToken(t *testing.T) {
	static := NewStatic("my-static-token")

	tok, err := static.Token(context.Background(), domain.PRRef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "my-static-token" {
		t.Fatalf("expected 'my-static-token', got %q", tok)
	}
}

func TestStatic_Token_EmptyToken(t *testing.T) {
	static := NewStatic("")

	tok, err := static.Token(context.Background(), domain.PRRef{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "" {
		t.Fatalf("expected empty, got %q", tok)
	}
}
