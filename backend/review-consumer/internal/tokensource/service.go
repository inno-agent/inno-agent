package tokensource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

// UserStore looks up a user_id by GitFlame username.
type UserStore interface {
	GetUserID(ctx context.Context, gitflameUsername string) (userID string, found bool, err error)
}

// Service implements domain.TokenSource using client credentials.
type Service struct {
	store        UserStore
	identityURL  string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu           sync.Mutex
	cachedToken  string
	cachedExpiry time.Time
}

// NewService creates a Service token source.
func NewService(store UserStore, identityURL, clientID, clientSecret string) *Service {
	return &Service{
		store:        store,
		identityURL:  strings.TrimRight(identityURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Token returns the service JWT and the assigner's user_id.
func (s *Service) Token(ctx context.Context, ref domain.PRRef) (token string, userID string, err error) {
	uid, found, err := s.store.GetUserID(ctx, ref.Assigner)
	if err != nil {
		return "", "", fmt.Errorf("tokensource: lookup user: %w: %w", domain.ErrTransient, err)
	}
	if !found {
		return "", "", domain.ErrNotOnboarded
	}

	tok, err := s.getServiceToken(ctx)
	if err != nil {
		return "", "", err
	}
	return tok, uid, nil
}

func (s *Service) getServiceToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken != "" && time.Until(s.cachedExpiry) > 5*time.Minute {
		return s.cachedToken, nil
	}

	body, err := json.Marshal(map[string]string{
		"client_id":     s.clientID,
		"client_secret": s.clientSecret,
	})
	if err != nil {
		return "", fmt.Errorf("tokensource: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.identityURL+"/identity/v1/service-token", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tokensource: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tokensource: service-token request: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		var result struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("tokensource: decode: %w: %w", domain.ErrTransient, err)
		}
		s.cachedToken = result.AccessToken
		s.cachedExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		return result.AccessToken, nil

	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("tokensource: service credentials rejected (status %d): %w",
			resp.StatusCode, domain.ErrPermanent)

	case resp.StatusCode >= 500:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("tokensource: identity server error %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrTransient)

	default:
		return "", fmt.Errorf("tokensource: unexpected status %d: %w",
			resp.StatusCode, domain.ErrPermanent)
	}
}
