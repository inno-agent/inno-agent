package mastra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

// Client calls the Mastra codegen agent service (/codegen endpoint).
// It mirrors backend/review-consumer/internal/mastra/client.go but targets
// the issue-codegen workflow instead of the review workflow.
type Client struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

// DefaultTimeout is the client-side budget for a /codegen call.
//
// Coupled triple: it must exceed the agent's CODEGEN_TIMEOUT_MS (default 900s
// for the multi-step agentic run) so the client does not abort before the agent
// answers, and the token freshness threshold must exceed BOTH so the delegated
// token cannot expire mid-run. So: agent 900s < this 1000s < freshness 20m.
const DefaultTimeout = 1000 * time.Second

func NewClient(baseURL, authToken string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		authToken: authToken,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

type codegenRequest struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	IssueNumber   int64  `json:"issueNumber"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
	IssueType     string `json:"issueType,omitempty"`
	Title         string `json:"title,omitempty"`
	Body          string `json:"body,omitempty"`
}

type codegenFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type codegenResponse struct {
	Summary  string        `json:"summary"`
	Files    []codegenFile `json:"files"`
	Verified bool          `json:"verified"`
}

// Generate calls the Mastra /codegen endpoint and returns the generated files.
// The Mastra service fetches repo context (AGENTS.md, README.md) and runs the
// code-generator agent itself; this client only forwards the issue ref plus
// whatever title/body the webhook already carried.
func (c *Client) Generate(ctx context.Context, ref domain.IssueRef, delegatedToken string) (*domain.GenerationResult, error) {
	payload := codegenRequest{
		Owner:         ref.Owner,
		Repo:          ref.Repo,
		IssueNumber:   ref.Index,
		DefaultBranch: ref.DefaultBranch,
		IssueType:     ref.IssueType,
		Title:         ref.Title,
		Body:          ref.Body,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("mastra: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/codegen", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("mastra: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	// The delegated user token rides in its own header: Authorization already
	// carries the shared service secret (actor), this carries the subject.
	if delegatedToken != "" {
		req.Header.Set("X-Delegated-Token", delegatedToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mastra: request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("mastra: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))

		// 401 almost always means an expired delegated token; a retry mints a
		// fresh one, so treat it as transient rather than dropping the task.
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
		}

		// 408/429 are 4xx but describe load, not a bad request: retry them.
		// Classifying 429 as permanent silently drops the issue on rate limit.
		if resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
		}

		// Other 4xx = permanent (bad request, auth, etc.)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}

		// 504 Gateway Timeout = transient (retry)
		if resp.StatusCode == http.StatusGatewayTimeout {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
		}

		// 500 with specific error patterns = permanent (model not found, invalid config)
		bodyLower := strings.ToLower(string(snippet))
		if strings.Contains(bodyLower, "model not found") ||
			strings.Contains(bodyLower, "invalid") ||
			strings.Contains(bodyLower, "not configured") {
			return nil, fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}

		// Other 5xx = transient (retry)
		return nil, fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var result codegenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Classify explicitly rather than letting the processor's default take
		// over: a 200 with an undecodable body is usually a truncated or
		// proxied response, which a retry can fix.
		return nil, fmt.Errorf("mastra: decode: %w: %w", domain.ErrTransient, err)
	}

	files := make([]domain.GeneratedFile, 0, len(result.Files))
	for _, f := range result.Files {
		if f.Path == "" {
			continue
		}
		files = append(files, domain.GeneratedFile{
			Path:    f.Path,
			Content: f.Content,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("mastra: codegen returned no files: %w", domain.ErrPermanent)
	}

	return &domain.GenerationResult{
		Files:    files,
		Summary:  result.Summary,
		Verified: result.Verified,
	}, nil
}
