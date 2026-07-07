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

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

var (
	_ domain.IssueSource   = (*Client)(nil)
	_ domain.CodePusher    = (*Client)(nil)
	_ domain.CommentPoster = (*Client)(nil)
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
			Timeout: 120 * time.Second,
		},
	}
}

type issueResponse struct {
	Title string          `json:"title"`
	Body  json.RawMessage `json:"body"`
}

func parseIssueBody(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	// GitFlame may return rich-text blocks as a JSON array/object.
	var blocks []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			switch {
			case b.Content != "":
				parts = append(parts, b.Content)
			case b.Text != "":
				parts = append(parts, b.Text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}

	return string(raw)
}

func (c *Client) GetIssue(ctx context.Context, ref domain.IssueRef) (string, string, error) {
	if c.baseURL == "" || c.token == "" {
		return "", "", fmt.Errorf("gitflame: not configured")
	}

	issueURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/issues/%d",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		ref.Index,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issueURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("gitflame: build issue request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("gitflame: issue request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: get issue returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", "", fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return "", "", fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var issue issueResponse
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return "", "", fmt.Errorf("gitflame: parse issue response: %w", err)
	}
	return issue.Title, parseIssueBody(issue.Body), nil
}

func escapePathSegments(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (c *Client) GetRawFile(ctx context.Context, ref domain.IssueRef, path string) (string, bool, error) {
	refName := ref.DefaultBranch
	if refName == "" {
		refName = "main"
	}

	rawURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/raw/%s?ref=%s",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		escapePathSegments(path),
		url.QueryEscape(refName),
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

type createFileRequest struct {
	Message       string `json:"message"`
	Content       string `json:"content"`
	Branch        string `json:"branch"`
	NewBranchName string `json:"new_branch_name,omitempty"`
}

func (c *Client) PushFiles(ctx context.Context, ref domain.IssueRef, branch string, files []domain.GeneratedFile, message string) error {
	if len(files) == 0 {
		return fmt.Errorf("gitflame: no files to push: %w", domain.ErrPermanent)
	}

	baseBranch := ref.DefaultBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	repoBase := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
	)

	for i, f := range files {
		reqBody := createFileRequest{
			Message: message,
			Content: base64.StdEncoding.EncodeToString([]byte(f.Content)),
			Branch:  baseBranch,
		}
		if i == 0 {
			reqBody.NewBranchName = branch
		} else {
			reqBody.Branch = branch
		}

		payload, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("gitflame: marshal create file %s: %w", f.Path, err)
		}

		fileURL := repoBase + "/contents/" + escapePathSegments(f.Path)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fileURL, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("gitflame: build create file request %s: %w", f.Path, err)
		}
		req.Header.Set("Authorization", "token "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("gitflame: create file %s failed: %w: %w", f.Path, domain.ErrTransient, err)
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			msg := fmt.Sprintf("gitflame: create file %s returned %d: %s", f.Path, resp.StatusCode, strings.TrimSpace(string(body)))
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
			}
			return fmt.Errorf("%s: %w", msg, domain.ErrTransient)
		}
	}

	return nil
}

func (c *Client) PostIssueComment(ctx context.Context, ref domain.IssueRef, body string) error {
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
