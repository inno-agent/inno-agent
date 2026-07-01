package gitflame

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

type prFile struct {
	Name string `json:"name"`
}

type fileDiff struct {
	FilePath string `json:"file_path"`
	Patch    string `json:"patch"` // base64-encoded unified diff hunk
	IsBinary bool   `json:"is_binary"`
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

	repoBase := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/pulls/%d",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(repo),
		prIndex,
	)

	files, err := c.listPRFiles(ctx, repoBase)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("%w: PR has no changed files", domain.ErrDiffUnavailable)
	}

	var sb strings.Builder
	for _, f := range files {
		patch, err := c.getFileDiff(ctx, repoBase, f.Name)
		if err != nil {
			return "", err
		}
		if patch == "" {
			continue
		}
		fmt.Fprintf(&sb, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n%s",
			f.Name, f.Name, f.Name, f.Name, patch)
		if !strings.HasSuffix(patch, "\n") {
			sb.WriteByte('\n')
		}
	}

	diff := sb.String()
	if diff == "" {
		return "", fmt.Errorf("%w: all files have empty patches", domain.ErrDiffUnavailable)
	}
	return diff, nil
}

func (c *Client) listPRFiles(ctx context.Context, repoBase string) ([]prFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoBase+"/files", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create files request: %w", domain.ErrDiffUnavailable, err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: gitflame files request failed: %w", domain.ErrDiffUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp, "files"); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read files response: %w", domain.ErrDiffUnavailable, err)
	}

	var files []prFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("%w: failed to parse files response: %w", domain.ErrDiffUnavailable, err)
	}
	return files, nil
}

func (c *Client) getFileDiff(ctx context.Context, repoBase, filename string) (string, error) {
	diffURL := repoBase + "/diff/" + url.PathEscape(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, diffURL, nil)
	if err != nil {
		return "", fmt.Errorf("%w: failed to create diff request for %s: %w", domain.ErrDiffUnavailable, filename, err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: gitflame diff request failed for %s: %w", domain.ErrDiffUnavailable, filename, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkStatus(resp, "diff/"+filename); err != nil {
		return "", err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", fmt.Errorf("%w: failed to read diff response for %s: %w", domain.ErrDiffUnavailable, filename, err)
	}

	var diffs []fileDiff
	if err := json.Unmarshal(body, &diffs); err != nil {
		return "", fmt.Errorf("%w: failed to parse diff response for %s: %w", domain.ErrDiffUnavailable, filename, err)
	}

	if len(diffs) == 0 || diffs[0].IsBinary || diffs[0].Patch == "" {
		return "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(diffs[0].Patch)
	if err != nil {
		return "", fmt.Errorf("%w: failed to decode patch for %s: %w", domain.ErrDiffUnavailable, filename, err)
	}
	return string(decoded), nil
}

// AcceptInvite confirms the bot account's pending collaborator invitation on owner/repo.
// GitFlame requires a collaborator invitation to be confirmed by the invitee before
// that account can be assigned as a PR reviewer.
func (c *Client) AcceptInvite(ctx context.Context, owner, repo string) error {
	if owner == "" || repo == "" {
		return fmt.Errorf("%w: owner and repo are required", domain.ErrValidation)
	}
	if c.baseURL == "" || c.token == "" {
		return fmt.Errorf("%w: gitflame is not configured", domain.ErrDiffUnavailable)
	}

	confirmURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/collaborators/confirm",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(owner),
		url.PathEscape(repo),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, confirmURL, nil)
	if err != nil {
		return fmt.Errorf("gitflame: failed to create confirm request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitflame: confirm invitation request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("gitflame: no pending invitation for %s/%s (404)", owner, repo)
	case http.StatusUnauthorized:
		return fmt.Errorf("gitflame: authentication failed (401) confirming invitation for %s/%s", owner, repo)
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("gitflame: confirm invitation for %s/%s returned %d: %s",
			owner, repo, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
}

func checkStatus(resp *http.Response, endpoint string) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: gitflame authentication failed (401) on %s", domain.ErrDiffUnavailable, endpoint)
	case http.StatusNotFound:
		return fmt.Errorf("%w: gitflame endpoint %s not found (404)", domain.ErrDiffUnavailable, endpoint)
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: gitflame %s returned %d: %s",
			domain.ErrDiffUnavailable, endpoint, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
}
