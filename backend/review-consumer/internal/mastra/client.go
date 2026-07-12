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
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func (c *Client) Review(ctx context.Context, ref domain.PRRef, token string) (string, error) {
	payload := map[string]interface{}{
		"owner":      ref.Owner,
		"repo":       ref.Repo,
		"pullNumber": ref.Index,
		"headSha":    ref.HeadSHA,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("mastra: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/review", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("mastra: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mastra: request failed: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("mastra: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return "", fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var result struct {
		ReviewMarkdown string `json:"review_markdown"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("mastra: decode: %w", err)
	}

	return result.ReviewMarkdown, nil
}
