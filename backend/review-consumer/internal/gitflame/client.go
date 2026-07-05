package gitflame

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

var (
	_ domain.SourceProvider = (*Client)(nil)
	_ domain.CommentPoster  = (*Client)(nil)
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

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
	Patch    string `json:"patch"`
	IsBinary bool   `json:"is_binary"`
}

func (c *Client) GetPRDiff(ctx context.Context, ref domain.PRRef) (string, error) {
	if c.baseURL == "" || c.token == "" {
		return "", fmt.Errorf("gitflame: not configured")
	}

	repoBase := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/pulls/%d",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		ref.Index,
	)

	files, err := c.listPRFiles(ctx, repoBase)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("gitflame: PR has no changed files")
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
		return "", fmt.Errorf("gitflame: all files have empty patches")
	}
	return diff, nil
}

// escapePathSegments escapes each segment of a slash-separated path
// individually so that slashes acting as path separators are preserved.
// e.g. "a/b.md" -> "a/b.md"  but  "a b/c d.md" -> "a%20b/c%20d.md"
func escapePathSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (c *Client) GetRawFile(ctx context.Context, ref domain.PRRef, path string) (string, bool, error) {
	rawURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/raw/%s?ref=%s",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		escapePathSegments(path),
		url.QueryEscape(ref.HeadSHA),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("gitflame: build raw request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("gitflame: raw request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: raw %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", false, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return "", false, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", false, fmt.Errorf("gitflame: read raw response: %w", err)
	}
	return string(body), true, nil
}

func (c *Client) PostPRComment(ctx context.Context, ref domain.PRRef, body string) error {
	commentURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/issues/%d/comments",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		ref.Index,
	)

	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("gitflame: marshal comment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, commentURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gitflame: build comment request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitflame: post comment failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: post comment returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}
	return nil
}

// RemoveRequestedReviewer cancels a pending review request for reviewer on the PR,
// so the bot can drop itself from the reviewer list once its review is posted.
func (c *Client) RemoveRequestedReviewer(ctx context.Context, ref domain.PRRef, reviewer string) error {
	reqURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/pulls/%d/requested_reviewers",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		ref.Index,
	)

	payload, err := json.Marshal(map[string][]string{"reviewers": {reviewer}})
	if err != nil {
		return fmt.Errorf("gitflame: marshal remove reviewer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gitflame: build remove reviewer request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitflame: remove reviewer request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: remove reviewer returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}
	return nil
}

func (c *Client) listPRFiles(ctx context.Context, repoBase string) ([]prFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoBase+"/files", nil)
	if err != nil {
		return nil, fmt.Errorf("gitflame: build files request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitflame: files request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: files returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return nil, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("gitflame: read files response: %w", err)
	}

	var files []prFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("gitflame: parse files response: %w", err)
	}
	return files, nil
}

func (c *Client) getFileDiff(ctx context.Context, repoBase, filename string) (string, error) {
	diffURL := repoBase + "/diff/" + url.PathEscape(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, diffURL, nil)
	if err != nil {
		return "", fmt.Errorf("gitflame: build diff request for %s: %w", filename, err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitflame: diff request failed for %s: %w: %w", filename, domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: diff %s returned %d: %s", filename, resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return "", fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", fmt.Errorf("gitflame: read diff response for %s: %w", filename, err)
	}

	var diffs []fileDiff
	if err := json.Unmarshal(body, &diffs); err != nil {
		return "", fmt.Errorf("gitflame: parse diff response for %s: %w", filename, err)
	}

	if len(diffs) == 0 || diffs[0].IsBinary || diffs[0].Patch == "" {
		return "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(diffs[0].Patch)
	if err != nil {
		return "", fmt.Errorf("gitflame: decode patch for %s: %w", filename, err)
	}
	return string(decoded), nil
}
