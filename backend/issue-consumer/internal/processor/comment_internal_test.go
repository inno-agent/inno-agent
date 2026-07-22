package processor

import (
	"errors"
	"strings"
	"testing"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

func TestBuildSuccessCommentReflectsVerified(t *testing.T) {
	verified := buildSuccessComment("br", 0, "", &domain.GenerationResult{
		Summary:      "did it",
		ChangedFiles: []domain.ChangedFile{{Path: "a.go", Status: "A"}},
		Verified:     true,
	}, nil)
	if !strings.Contains(strings.ToLower(verified), "verified") {
		t.Errorf("verified comment lacks a verified marker:\n%s", verified)
	}

	unverified := buildSuccessComment("br", 0, "", &domain.GenerationResult{
		Summary:      "did it",
		ChangedFiles: []domain.ChangedFile{{Path: "a.go", Status: "A"}},
		Verified:     false,
	}, nil)
	if !strings.Contains(strings.ToLower(unverified), "not") && !strings.Contains(unverified, "⚠") {
		t.Errorf("unverified comment lacks a not-verified warning:\n%s", unverified)
	}
	// The two must differ — a comment that ignores Verified would be identical.
	if verified == unverified {
		t.Error("verified and unverified comments are identical; Verified was ignored")
	}
}

// A failed PR creation still leaves working code pushed to a branch — the
// comment must tell the user to open the PR by hand instead of staying quiet.
func TestBuildSuccessCommentSurfacesPRFailure(t *testing.T) {
	comment := buildSuccessComment("br", 0, "alice", &domain.GenerationResult{
		Summary:      "did it",
		ChangedFiles: []domain.ChangedFile{{Path: "a.go", Status: "A"}},
		Verified:     true,
	}, errors.New("gitflame: create pr returned 500"))

	if !strings.Contains(comment, "gitflame: create pr returned 500") {
		t.Errorf("comment does not surface the PR creation error:\n%s", comment)
	}
	if !strings.Contains(comment, "br") {
		t.Errorf("comment does not point back to the branch to open the PR from:\n%s", comment)
	}
}
