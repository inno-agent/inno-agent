package gitflame

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

var prConflictNumberRE = regexp.MustCompile(`issue_id:\s*(\d+)`)

var (
	_ domain.IssueSource        = (*Client)(nil)
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
		return "", "", fmt.Errorf("gitflame: not configured: %w", domain.ErrPermanent)
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
	Title string            `json:"title"`
	From  string            `json:"from"`
	To    string            `json:"to"`
	Body  []prRichTextBlock `json:"body,omitempty"`
}

type prRichTextBlock struct {
	Body string `json:"body"`
	Mime string `json:"mime"`
	Size int    `json:"size"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type createPullRequestResponse struct {
	Number int64 `json:"number"`
	Index  int64 `json:"index"`
}

func buildPRBody(text string) []prRichTextBlock {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	paragraphs := strings.Split(text, "\n\n")
	blocks := make([]prRichTextBlock, 0, len(paragraphs))
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		escaped := html.EscapeString(p)
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		blocks = append(blocks, prRichTextBlock{
			Body: "<p>" + escaped + "</p>",
			Mime: "text",
			Size: 1,
			Name: "text",
			Type: "text",
		})
	}
	if len(blocks) == 0 {
		return nil
	}
	return blocks
}

func (c *Client) CreatePullRequest(
	ctx context.Context,
	ref domain.IssueRef,
	headBranch, title, body string,
	reviewers []string,
) (int64, error) {
	if c.baseURL == "" || c.token == "" {
		return 0, fmt.Errorf("gitflame: not configured: %w", domain.ErrPermanent)
	}

	baseBranch := ref.DefaultBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	reqBody := createPullRequestRequest{
		Title: title,
		From:  headBranch,
		To:    baseBranch,
		Body:  buildPRBody(body),
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

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	prIndex, resolveErr := c.resolvePullRequestNumber(ctx, ref, resp.StatusCode, respBody, headBranch, baseBranch)
	if resolveErr != nil {
		return 0, resolveErr
	}

	if len(reviewers) > 0 {
		_ = c.addPullRequestReviewers(ctx, ref, prIndex, reviewers)
	}

	if linkErr := c.linkPullRequestToIssue(ctx, ref, prIndex); linkErr != nil {
		log.Printf("gitflame: link pull request #%d to issue %s/%s#%d failed: %v",
			prIndex, ref.Owner, ref.Repo, ref.Index, linkErr)
	}

	return prIndex, nil
}

// fullIssue is GitFlame's issue read shape. It carries every field the edit
// endpoint below must round-trip — see linkPullRequestToIssue.
type fullIssue struct {
	Title        string            `json:"title"`
	Body         []issueBodyBlock  `json:"body"`
	Assignees    []issueUser       `json:"assignees"`
	Labels       []issueLabel      `json:"labels"`
	Milestone    *issueMilestone   `json:"milestone"`
	Dependencies []issueDependency `json:"dependencies"`
}

type issueBodyBlock struct {
	Body string `json:"body"`
	Mime string `json:"mime"`
	Size int    `json:"size"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type issueUser struct {
	Login string `json:"login"`
}

type issueLabel struct {
	ID int64 `json:"id"`
}

type issueMilestone struct {
	ID int64 `json:"id"`
}

type issueDependency struct {
	Number int64 `json:"number"`
}

// editIssueRequest mirrors GitFlame's own issue-edit form submission: the
// endpoint is PATCH-as-full-replace, not a merge (confirmed from a real
// network trace of the GitFlame web UI), so every editable field read via
// getFullIssue must be sent back unchanged except the one being modified.
type editIssueRequest struct {
	Title        string           `json:"title"`
	Body         []issueBodyBlock `json:"body"`
	Assignees    []string         `json:"assignees"`
	Labels       []int64          `json:"labels"`
	Milestone    int64            `json:"milestone"`
	Dependencies []int64          `json:"dependencies"`
	Type         string           `json:"type"`
}

func (c *Client) getFullIssue(ctx context.Context, ref domain.IssueRef) (*fullIssue, error) {
	issueURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/issues/%d",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		ref.Index,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issueURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gitflame: build get issue request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitflame: get issue failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: get issue returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return nil, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var issue fullIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, fmt.Errorf("gitflame: parse full issue: %w", err)
	}
	return &issue, nil
}

// linkPullRequestToIssue adds prNumber to the issue's dependencies so
// GitFlame's UI shows the PR as linked — GitFlame does not auto-link from
// "Closes #N" text in the PR body the way GitHub/Gitea do; the link is a
// separate explicit field on the issue, set here via read-modify-write.
// Best-effort: called with its error discarded — a missing link is cosmetic,
// not worth failing PR creation over.
func (c *Client) linkPullRequestToIssue(ctx context.Context, ref domain.IssueRef, prNumber int64) error {
	issue, err := c.getFullIssue(ctx, ref)
	if err != nil {
		return err
	}

	deps := make([]int64, 0, len(issue.Dependencies)+1)
	linked := false
	for _, d := range issue.Dependencies {
		deps = append(deps, d.Number)
		if d.Number == prNumber {
			linked = true
		}
	}
	if !linked {
		deps = append(deps, prNumber)
	}

	var milestoneID int64
	if issue.Milestone != nil {
		milestoneID = issue.Milestone.ID
	}

	labels := make([]int64, 0, len(issue.Labels))
	for _, l := range issue.Labels {
		labels = append(labels, l.ID)
	}

	assignees := make([]string, 0, len(issue.Assignees))
	for _, a := range issue.Assignees {
		assignees = append(assignees, a.Login)
	}

	reqBody := editIssueRequest{
		Title:        issue.Title,
		Body:         issue.Body,
		Assignees:    assignees,
		Labels:       labels,
		Milestone:    milestoneID,
		Dependencies: deps,
		Type:         "issue",
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("gitflame: marshal edit issue: %w", err)
	}

	issueURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/issues/%d",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		ref.Index,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, issueURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gitflame: build edit issue request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitflame: edit issue failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("gitflame: edit issue returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}
	return nil
}

func (c *Client) resolvePullRequestNumber(
	ctx context.Context,
	ref domain.IssueRef,
	statusCode int,
	respBody []byte,
	headBranch, baseBranch string,
) (int64, error) {
	switch statusCode {
	case http.StatusCreated, http.StatusOK:
		if prIndex := parsePullRequestNumber(respBody); prIndex > 0 {
			return prIndex, nil
		}
		// GitFlame sometimes returns 201 with an empty body; look up the PR we just created.
		if prIndex, err := c.findOpenPullRequestByHead(ctx, ref, headBranch, baseBranch); err == nil {
			return prIndex, nil
		} else if len(bytesTrimSpace(respBody)) == 0 {
			return 0, err
		}
		return 0, fmt.Errorf("gitflame: create pull request returned no number: %w", domain.ErrPermanent)

	case http.StatusConflict:
		if prIndex := parseExistingPRNumber(respBody); prIndex > 0 {
			return prIndex, nil
		}
		if prIndex, err := c.findOpenPullRequestByHead(ctx, ref, headBranch, baseBranch); err == nil {
			return prIndex, nil
		}
		msg := fmt.Sprintf("gitflame: create pull request returned %d: %s", statusCode, strings.TrimSpace(string(respBody)))
		return 0, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)

	default:
		if statusCode >= 400 && statusCode < 500 {
			msg := fmt.Sprintf("gitflame: create pull request returned %d: %s", statusCode, strings.TrimSpace(string(respBody)))
			return 0, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		msg := fmt.Sprintf("gitflame: create pull request returned %d: %s", statusCode, strings.TrimSpace(string(respBody)))
		return 0, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}
}

func parsePullRequestNumber(respBody []byte) int64 {
	respBody = bytesTrimSpace(respBody)
	if len(respBody) == 0 {
		return 0
	}

	var pr createPullRequestResponse
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return 0
	}
	if pr.Number != 0 {
		return pr.Number
	}
	return pr.Index
}

func parseExistingPRNumber(respBody []byte) int64 {
	var errBody struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &errBody); err != nil {
		return 0
	}
	matches := prConflictNumberRE.FindStringSubmatch(errBody.Message)
	if len(matches) < 2 {
		return 0
	}
	n, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

type pullListItem struct {
	Number int64 `json:"number"`
	Head   struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (c *Client) findOpenPullRequestByHead(
	ctx context.Context,
	ref domain.IssueRef,
	headBranch, baseBranch string,
) (int64, error) {
	listURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/pulls?state=open&limit=50",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return 0, fmt.Errorf("gitflame: build pull list request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gitflame: list pull requests failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("gitflame: list pull requests returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return 0, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return 0, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var list struct {
		List []pullListItem `json:"list"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, fmt.Errorf("gitflame: parse pull list response: %w", err)
	}

	for _, pr := range list.List {
		if pr.Head.Ref != headBranch {
			continue
		}
		if baseBranch != "" && pr.Base.Ref != baseBranch {
			continue
		}
		if pr.Number > 0 {
			return pr.Number, nil
		}
	}

	return 0, fmt.Errorf("gitflame: no open pull request for branch %s: %w", headBranch, domain.ErrPermanent)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func (c *Client) addPullRequestReviewers(ctx context.Context, ref domain.IssueRef, prIndex int64, reviewers []string) error {
	reviewersURL := fmt.Sprintf(
		"%s/api/v1/repos/%s/%s/pulls/%d/requested_reviewers",
		strings.TrimRight(c.baseURL, "/"),
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		prIndex,
	)

	payload, err := json.Marshal(map[string][]string{"reviewers": reviewers})
	if err != nil {
		return fmt.Errorf("gitflame: marshal reviewers: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reviewersURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gitflame: build reviewers request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitflame: add reviewers failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg := fmt.Sprintf("gitflame: add reviewers returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
	}
	return fmt.Errorf("%s: %w", msg, domain.ErrTransient)
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
