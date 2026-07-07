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

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

type UserStore interface {
	GetUserID(ctx context.Context, gitflameUsername string) (userID string, found bool, err error)
}

type cachedDelegate struct {
	token  string
	expiry time.Time
}

type Service struct {
	store        UserStore
	identityURL  string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu            sync.Mutex
	cachedToken   string
	cachedExpiry  time.Time
	delegateCache map[string]cachedDelegate
}

func NewService(store UserStore, identityURL, clientID, clientSecret string) *Service {
	return &Service{
		store:         store,
		identityURL:   strings.TrimRight(identityURL, "/"),
		clientID:      clientID,
		clientSecret:  clientSecret,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		delegateCache: make(map[string]cachedDelegate),
	}
}

func (s *Service) Token(ctx context.Context, ref domain.IssueRef) (string, error) {
	uid, found, err := s.store.GetUserID(ctx, ref.Assigner)
	if err != nil {
		return "", fmt.Errorf("tokensource: lookup user: %w: %w", domain.ErrTransient, err)
	}
	if !found {
		return "", domain.ErrNotOnboarded
	}

	svcTok, err := s.getServiceToken(ctx)
	if err != nil {
		return "", err
	}
	return s.exchangeToken(ctx, uid, svcTok)
}

func (s *Service) getServiceToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.cachedToken != "" && time.Until(s.cachedExpiry) > 5*time.Minute {
		tok := s.cachedToken
		s.mu.Unlock()
		return tok, nil
	}
	s.mu.Unlock()

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
		return "", fmt.Errorf("tokensource: build service-token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tokensource: service-token: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		var result struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("tokensource: decode service-token: %w: %w", domain.ErrTransient, err)
		}
		s.mu.Lock()
		s.cachedToken = result.AccessToken
		s.cachedExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		s.mu.Unlock()
		return result.AccessToken, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("tokensource: service credentials rejected (status %d): %w",
			resp.StatusCode, domain.ErrPermanent)
	case resp.StatusCode >= 500:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("tokensource: identity error %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrTransient)
	default:
		return "", fmt.Errorf("tokensource: unexpected service-token status %d: %w",
			resp.StatusCode, domain.ErrPermanent)
	}
}

func (s *Service) exchangeToken(ctx context.Context, userID, actorToken string) (string, error) {
	s.mu.Lock()
	if cd, ok := s.delegateCache[userID]; ok && time.Until(cd.expiry) > 5*time.Minute {
		tok := cd.token
		s.mu.Unlock()
		return tok, nil
	}
	s.mu.Unlock()

	body, err := json.Marshal(map[string]string{
		"grant_type":    "urn:ietf:params:oauth:grant-type:token-exchange",
		"actor_token":   actorToken,
		"subject_token": userID,
	})
	if err != nil {
		return "", fmt.Errorf("tokensource: marshal exchange: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.identityURL+"/identity/v1/token", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tokensource: build exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tokensource: token exchange: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		var result struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("tokensource: decode exchange: %w: %w", domain.ErrTransient, err)
		}
		expiry := time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
		s.mu.Lock()
		s.delegateCache[userID] = cachedDelegate{token: result.AccessToken, expiry: expiry}
		s.mu.Unlock()
		return result.AccessToken, nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", fmt.Errorf("tokensource: exchange rejected (status %d): %w",
			resp.StatusCode, domain.ErrPermanent)
	case resp.StatusCode >= 500:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("tokensource: exchange server error %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrTransient)
	default:
		return "", fmt.Errorf("tokensource: unexpected exchange status %d: %w",
			resp.StatusCode, domain.ErrPermanent)
	}
}
