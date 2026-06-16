package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type Result struct {
	UserID        string   `json:"user_id"`
	Tier          string   `json:"tier"`
	Allowed       bool     `json:"allowed"`
	AllowedModels []string `json:"allowed_models"`
}

// Authorize asks identity whether the token is valid and (optionally) whether
// the given model is permitted. An empty model only checks the token and
// returns the policy list.
func (c *Client) Authorize(ctx context.Context, token, model string) (*Result, error) {
	payload, err := json.Marshal(map[string]string{"token": token, "model": model})
	if err != nil {
		return nil, fmt.Errorf("auth: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/identity/v1/authorize", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("auth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: identity status %d", resp.StatusCode)
	}

	var out Result
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("auth: decode: %w", err)
	}
	return &out, nil
}

var ErrUnauthorized = fmt.Errorf("unauthorized")
