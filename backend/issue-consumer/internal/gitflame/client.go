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
	_ domain.IssueSource        = (*Client)(nil)
	_ domain.CodePusher         = (*Client)(nil)
	_ domain.PullRequestCreator = (*Client)(nil)
	_ domain.CommentPoster      = (*Client)(nil)
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
	return issue.Title, ParseIssueBody(issue.Body), nil
}

type createPullRequestRequest struct {
	Title     string   `json:"title"`
	Head      string   `json:"head"`
	Base      string   `json:"base"`
	Body      string   `json:"body"`
	Reviewers []string `json:"reviewers,omitempty"`
}

type createPullRequestResponse struct {
	Number int64 `json:"number"`
	Index  int64 `json:"index"`
}

func (c *Client) CreatePullRequest(
	ctx context.Context,
	ref domain.IssueRef,
	headBranch, title, body string,
	reviewers []string,
) (int64, error) {
	if c.baseURL == "" || c.token == "" {
		return 0, fmt.Errorf("gitflame: not configured")
	}

	baseBranch := ref.DefaultBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	reqBody := createPullRequestRequest{
		Title:     title,
		Head:      headBranch,
		Base:      baseBranch,
		Body:      body,
		Reviewers: reviewers,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("gitflame: marshal pull request: %w", err)
	}

	prURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/pulls",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, prURL, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("gitflame: build pull request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gitflame: create pull request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: create pull request returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return 0, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return 0, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var pr createPullRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return 0, fmt.Errorf("gitflame: parse pull request response: %w", err)
	}
	if pr.Number != 0 {
		return pr.Number, nil
	}
	return pr.Index, nil
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
	Message   string `json:"message"`
	Content   string `json:"content"`
	Branch    string `json:"branch"`
	NewBranch string `json:"new_branch,omitempty"`
}

type createBranchRequest struct {
	NewBranchName string `json:"new_branch_name"`
	OldBranchName string `json:"old_branch_name"`
}

func (c *Client) ensureBranch(ctx context.Context, ref domain.IssueRef, branch, baseBranch string) error {
	payload, err := json.Marshal(createBranchRequest{
		NewBranchName: branch,
		OldBranchName: baseBranch,
	})
	if err != nil {
		return fmt.Errorf("gitflame: marshal branch request: %w", err)
	}

	branchURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/branches",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, branchURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gitflame: build branch request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitflame: create branch failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := fmt.Sprintf("gitflame: create branch %s returned %d: %s", branch, resp.StatusCode, strings.TrimSpace(string(body)))

	// Branch already exists — safe to push more commits to it.
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusUnprocessableEntity {
		return nil
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
	}
	return fmt.Errorf("%s: %w", msg, domain.ErrTransient)
}

func (c *Client) PushFiles(ctx context.Context, ref domain.IssueRef, branch string, files []domain.GeneratedFile, message string) error {
	if len(files) == 0 {
		return fmt.Errorf("gitflame: no files to push: %w", domain.ErrPermanent)
	}

	baseBranch := ref.DefaultBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	if err := c.ensureBranch(ctx, ref, branch, baseBranch); err != nil {
		return err
	}

	repoBase := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
	)

	for _, f := range files {
		reqBody := createFileRequest{
			Message: message,
			Content: base64.StdEncoding.EncodeToString([]byte(f.Content)),
			Branch:  branch,
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
