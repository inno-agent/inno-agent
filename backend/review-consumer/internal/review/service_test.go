package review_test

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/review"
)

type fakeSource struct {
	diff        string
	diffErr     error
	files       map[string]string
	fileFound   map[string]bool
	description string
}

func (f *fakeSource) GetPRDiff(_ context.Context, _ domain.PRRef) (string, error) {
	return f.diff, f.diffErr
}

func (f *fakeSource) GetRawFile(_ context.Context, _ domain.PRRef, path string) (string, bool, error) {
	if f.files == nil {
		return "", false, nil
	}
	content, ok := f.files[path]
	return content, ok, nil
}

func (f *fakeSource) GetPRDescription(_ context.Context, _ domain.PRRef) (string, error) {
	return f.description, nil
}

type fakeLLM struct {
	answer   string
	err      error
	lastMsgs []domain.LLMMessage
}

func (f *fakeLLM) Chat(_ context.Context, msgs []domain.LLMMessage, _ string) (string, error) {
	f.lastMsgs = msgs
	return f.answer, f.err
}

type fakeTokenSource struct{ token string }

func (f *fakeTokenSource) Token(_ context.Context, _ domain.PRRef) (string, error) {
	return f.token, nil
}

func TestReview_IncludesAgentsMD(t *testing.T) {
	src := &fakeSource{
		diff:  "diff --git a/foo.go",
		files: map[string]string{"AGENTS.md": "# Agents", "README.md": "# Readme"},
	}
	llmClient := &fakeLLM{answer: "# Summary\nLGTM"}
	tok := &fakeTokenSource{token: "tok123"}

	svc := review.NewService(src, llmClient, tok, "model", zap.NewNop())
	result, err := svc.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1, HeadSHA: "abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "# Summary\nLGTM" {
		t.Fatalf("unexpected result: %q", result)
	}

	if len(llmClient.lastMsgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(llmClient.lastMsgs))
	}
	userMsg := llmClient.lastMsgs[1].Content
	if !strings.Contains(userMsg, "# Agents") {
		t.Error("expected AGENTS.md content in user message")
	}
	if !strings.Contains(userMsg, "# Readme") {
		t.Error("expected README.md content in user message")
	}
	if !strings.Contains(userMsg, "diff --git") {
		t.Error("expected diff in user message")
	}
}

func TestReview_AbsentFilesNoted(t *testing.T) {
	src := &fakeSource{diff: "diff --git a/foo.go"}
	llmClient := &fakeLLM{answer: "# Summary\nOK"}
	tok := &fakeTokenSource{token: "tok"}

	svc := review.NewService(src, llmClient, tok, "", zap.NewNop())
	_, err := svc.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userMsg := llmClient.lastMsgs[1].Content
	if strings.Count(userMsg, "(absent)") < 2 {
		t.Errorf("expected both AGENTS.md and README.md noted as absent, got: %q", userMsg)
	}
}
