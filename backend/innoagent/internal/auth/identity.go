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

var ErrUnauthorized = fmt.Errorf("unauthorized")

// Validate checks the token against identity and returns the user_id.
func (c *Client) Validate(ctx context.Context, token string) (string, error) {
	payload, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return "", fmt.Errorf("auth: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/identity/v1/validate", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("auth: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth: identity status %d", resp.StatusCode)
	}

	var out struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("auth: decode: %w", err)
	}
	return out.UserID, nil
}
