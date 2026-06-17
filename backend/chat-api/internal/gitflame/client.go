package gitflame

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
// prID format: "owner/repo/pr_index" (e.g. "my-org/backend-service/42")
func (c *Client) GetPRDiff(ctx context.Context, prID string) (string, error) {
	if prID == "" {
		return "", fmt.Errorf("gitflame: pr_id is required")
	}
	if c.baseURL == "" || c.token == "" {
		return "", fmt.Errorf("%w: gitflame is not configured", domain.ErrDiffUnavailable)
	}

	parts := strings.Split(prID, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("gitflame: invalid pr_id format, expected owner/repo/pr_index")
	}

	owner := parts[0]
	repo := parts[1]
	prIndex := parts[2]

	// GET /repos/{owner}/{repo}/pulls/{index}.diff
	reqURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s.diff",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(repo),
		prIndex,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("gitflame: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: gitflame request failed: %w", domain.ErrDiffUnavailable, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10 MB limit
	if err != nil {
		return "", fmt.Errorf("%w: failed to read response: %w", domain.ErrDiffUnavailable, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		diff := string(body)
		if diff == "" {
			return "", fmt.Errorf("%w: empty diff from gitflame", domain.ErrDiffUnavailable)
		}
		return diff, nil

	case http.StatusNotFound:
		return "", fmt.Errorf("gitflame: PR not found (404): %s/%s/%s", owner, repo, prIndex)

	case http.StatusUnauthorized:
		return "", fmt.Errorf("%w: gitflame authentication failed (401)", domain.ErrDiffUnavailable)

	case http.StatusForbidden:
		return "", fmt.Errorf("%w: gitflame access denied (403)", domain.ErrDiffUnavailable)

	default:
		return "", fmt.Errorf(
			"%w: gitflame returned status %d: %s",
			domain.ErrDiffUnavailable,
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}
}