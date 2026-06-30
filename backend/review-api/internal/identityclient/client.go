package identityclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client wraps the identity service HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Client targeting baseURL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GrantDelegation creates a delegation grant in identity.
// userToken is the user's Bearer token (forwarded as-is from the install request).
// clientID is the service that will act on the user's behalf.
func (c *Client) GrantDelegation(ctx context.Context, userToken, clientID string) error {
	body, err := json.Marshal(map[string]string{"client_id": clientID})
	if err != nil {
		return fmt.Errorf("identity client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/identity/v1/delegation-grant", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("identity client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("identity client: delegation grant: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("identity client: delegation grant: unexpected status %d", resp.StatusCode)
	}
	return nil
}
