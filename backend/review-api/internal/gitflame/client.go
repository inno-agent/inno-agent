package gitflame

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/domain"
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
		return "", fmt.Errorf("%w: invalid pr_id format, expected owner/repo/pr_index", domain.ErrValidation)
	}

	owner := parts[0]
	repo := parts[1]

	prIndex, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", fmt.Errorf("%w: pr_index must be integer, got %q", domain.ErrValidation, parts[2])
	}

	reqURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d.diff",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(repo),
		prIndex,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("%w: failed to create request: %w", domain.ErrDiffUnavailable, err)
	}

	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: gitflame request failed: %w", domain.ErrDiffUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		if err != nil {
			return "", fmt.Errorf("%w: failed to read response: %w", domain.ErrDiffUnavailable, err)
		}
		diff := string(body)
		if diff == "" {
			return "", fmt.Errorf("%w: empty diff from gitflame", domain.ErrDiffUnavailable)
		}
		return diff, nil

	case http.StatusUnauthorized:
		return "", fmt.Errorf("%w: gitflame authentication failed (401)", domain.ErrDiffUnavailable)

	case http.StatusNotFound:
		return "", fmt.Errorf("%w: gitflame PR %s/%s/%d not found", domain.ErrDiffUnavailable, owner, repo, prIndex)

	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf(
			"%w: gitflame returned status %d: %s",
			domain.ErrDiffUnavailable,
			resp.StatusCode,
			strings.TrimSpace(string(snippet)),
		)
	}
}
