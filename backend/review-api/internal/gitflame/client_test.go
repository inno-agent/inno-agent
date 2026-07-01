package gitflame_test

import (
	"context"
	"errors"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/gitflame"
)

func TestClient_GetPRDiff_NotConfigured(t *testing.T) {
	client := gitflame.NewClient("", "")

	_, err := client.GetPRDiff(context.Background(), "owner/repo/1")
	if !errors.Is(err, domain.ErrDiffUnavailable) {
		t.Fatalf("expected ErrDiffUnavailable, got %v", err)
	}
}

func TestClient_GetPRDiff_InvalidFormat(t *testing.T) {
	client := gitflame.NewClient("https://gitflame.example", "token")

	_, err := client.GetPRDiff(context.Background(), "123")
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestClient_GetPRDiff_ConfiguredButUnavailable(t *testing.T) {
	client := gitflame.NewClient("https://gitflame.example", "token")

	_, err := client.GetPRDiff(context.Background(), "owner/repo/1")
	if !errors.Is(err, domain.ErrDiffUnavailable) {
		t.Fatalf("expected ErrDiffUnavailable, got %v", err)
	}
}

func TestClient_AcceptInvite_NotConfigured(t *testing.T) {
	client := gitflame.NewClient("", "")

	err := client.AcceptInvite(context.Background(), "owner", "repo")
	if !errors.Is(err, domain.ErrDiffUnavailable) {
		t.Fatalf("expected ErrDiffUnavailable, got %v", err)
	}
}

func TestClient_AcceptInvite_MissingOwnerOrRepo(t *testing.T) {
	client := gitflame.NewClient("https://gitflame.example", "token")

	if err := client.AcceptInvite(context.Background(), "", "repo"); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if err := client.AcceptInvite(context.Background(), "owner", ""); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}
