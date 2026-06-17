package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/domain"
)

type mockDiffProvider struct {
	diff string
	err  error
}

func (m *mockDiffProvider) GetPRDiff(_ context.Context, _ string) (string, error) {
	return m.diff, m.err
}

type mockLLMProvider struct {
	answer string
	err    error
	last   []domain.LLMMessage
}

func (m *mockLLMProvider) Chat(_ context.Context, messages []domain.LLMMessage) (string, error) {
	m.last = messages
	return m.answer, m.err
}

func TestReviewService_WithProvidedDiff_SkipsDiffProvider(t *testing.T) {
	diffProvider := &mockDiffProvider{err: errors.New("should not be called")}
	llm := &mockLLMProvider{answer: "# Summary\nAll good."}
	svc := NewReviewService(diffProvider, llm, zap.NewNop())

	review, err := svc.ReviewPR(context.Background(), "my-org/repo/1", "diff --git a/main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(review, "# Summary") {
		t.Fatalf("expected markdown review, got %q", review)
	}
	if len(llm.last) != 2 {
		t.Fatalf("expected 2 llm messages, got %d", len(llm.last))
	}
	if !strings.Contains(llm.last[1].Content, "diff --git a/main.go") {
		t.Fatalf("expected diff in user message, got %q", llm.last[1].Content)
	}
}

func TestReviewService_WithoutDiff_UsesDiffProvider(t *testing.T) {
	diffProvider := &mockDiffProvider{diff: "diff --git a/auth.go"}
	llm := &mockLLMProvider{answer: "# Summary\nReview complete."}
	svc := NewReviewService(diffProvider, llm, zap.NewNop())

	review, err := svc.ReviewPR(context.Background(), "my-org/repo/2", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(review, "Review complete") {
		t.Fatalf("expected markdown review, got %q", review)
	}
}

func TestReviewService_WithoutDiff_ReturnsUnavailable(t *testing.T) {
	diffProvider := &mockDiffProvider{err: domain.ErrDiffUnavailable}
	svc := NewReviewService(diffProvider, &mockLLMProvider{}, zap.NewNop())

	_, err := svc.ReviewPR(context.Background(), "my-org/repo/3", "")
	if !errors.Is(err, domain.ErrDiffUnavailable) {
		t.Fatalf("expected ErrDiffUnavailable, got %v", err)
	}
}
