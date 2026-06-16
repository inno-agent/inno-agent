package gitflame

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

var _ domain.DiffProvider = (*Client)(nil)

// Client fetches pull request diffs from GitFlame.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a GitFlame diff provider.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetPRDiff returns the unified diff for the given pull request.
func (c *Client) GetPRDiff(ctx context.Context, prID string) (string, error) {
	if prID == "" {
		return "", fmt.Errorf("gitflame: pr_id is required")
	}
	if c.baseURL == "" || c.token == "" {
		return "", fmt.Errorf("%w: gitflame is not configured", domain.ErrDiffUnavailable)
	}

	_ = ctx
	_ = c.httpClient

	return "", fmt.Errorf("%w: gitflame diff fetch is not available; provide diff in the request body", domain.ErrDiffUnavailable)
}
