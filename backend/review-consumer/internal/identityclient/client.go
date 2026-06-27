// Package identityclient is a thin client for the identity service's generic
// refresh endpoint.
package identityclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

// ErrGrantDead is returned when the identity service rejects the refresh token
// (401) — the grant is dead and the user must re-onboard.
var ErrGrantDead = errors.New("refresh grant dead")

// Client calls the identity service /identity/v1/refresh endpoint.
type Client struct {
	identityURL string
	httpClient  *http.Client
}

// New creates an identity Client.
func New(identityURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		identityURL: strings.TrimRight(identityURL, "/"),
		httpClient:  httpClient,
	}
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`         // seconds
	RefreshToken     string `json:"refresh_token"`      // rotated
	RefreshExpiresIn int    `json:"refresh_expires_in"` // seconds
}

// Refresh exchanges a refresh token for a fresh access token and a rotated
// refresh token. accessExpiry is the absolute time the access token expires.
//
// A 401 returns ErrGrantDead; 5xx/transport errors wrap domain.ErrTransient.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (access string, newRefresh string, accessExpiry time.Time, err error) {
	body, err := json.Marshal(refreshRequest{RefreshToken: refreshToken})
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("identityclient: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.identityURL+"/identity/v1/refresh", bytes.NewReader(body))
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("identityclient: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("identityclient: request: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		var rr refreshResponse
		if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
			return "", "", time.Time{}, fmt.Errorf("identityclient: decode: %w: %w", domain.ErrTransient, err)
		}

		exp := time.Now().Add(time.Duration(rr.ExpiresIn) * time.Second)
		return rr.AccessToken, rr.RefreshToken, exp, nil

	case resp.StatusCode == http.StatusUnauthorized:
		return "", "", time.Time{}, ErrGrantDead

	case resp.StatusCode >= 500:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", "", time.Time{}, fmt.Errorf("identityclient: status %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrTransient)

	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", "", time.Time{}, fmt.Errorf("identityclient: status %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrPermanent)
	}
}
