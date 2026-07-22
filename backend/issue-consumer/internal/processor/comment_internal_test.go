package processor

import (
	"strings"
	"testing"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

func TestBuildSuccessCommentReflectsVerified(t *testing.T) {
	verified := buildSuccessComment("br", 0, "", &domain.GenerationResult{
		Summary:  "did it",
		Files:    []domain.GeneratedFile{{Path: "a.go"}},
		Verified: true,
	})
	if !strings.Contains(strings.ToLower(verified), "verified") {
		t.Errorf("verified comment lacks a verified marker:\n%s", verified)
	}

	unverified := buildSuccessComment("br", 0, "", &domain.GenerationResult{
		Summary:  "did it",
		Files:    []domain.GeneratedFile{{Path: "a.go"}},
		Verified: false,
	})
	if !strings.Contains(strings.ToLower(unverified), "not") && !strings.Contains(unverified, "⚠") {
		t.Errorf("unverified comment lacks a not-verified warning:\n%s", unverified)
	}
	// The two must differ — a comment that ignores Verified would be identical.
	if verified == unverified {
		t.Error("verified and unverified comments are identical; Verified was ignored")
	}
}
