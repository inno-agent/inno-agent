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

var _ domain.TokenSource = (*Identity)(nil)

// Identity fetches short-TTL aicore tokens from the identity service mint
// endpoint on behalf of the assigner, caching them until ~30 s before expiry.
type Identity struct {
	identityURL   string
	serviceSecret string
	httpClient    *http.Client

	mu    sync.Mutex
	cache map[string]cachedToken
}

type cachedToken struct {
	token string
	exp   time.Time
}

// NewIdentity creates an Identity TokenSource.
func NewIdentity(identityURL, serviceSecret string, httpClient *http.Client) *Identity {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Identity{
		identityURL:   strings.TrimRight(identityURL, "/"),
		serviceSecret: serviceSecret,
		httpClient:    httpClient,
		cache:         make(map[string]cachedToken),
	}
}

// Token returns a valid aicore token for the assigner named in ref.Assigner.
// It caches tokens and refreshes them ~30 s before expiry.
func (id *Identity) Token(ctx context.Context, ref domain.PRRef) (string, error) {
	assigner := ref.Assigner
	if assigner == "" {
		return "", fmt.Errorf("token: assigner is empty: %w", domain.ErrPermanent)
	}

	// Fast path: return cached token if still fresh.
	id.mu.Lock()
	if ct, ok := id.cache[assigner]; ok && time.Now().Before(ct.exp) {
		tok := ct.token
		id.mu.Unlock()

		return tok, nil
	}

	id.mu.Unlock()

	// Slow path: call identity.
	tok, exp, err := id.mint(ctx, assigner)
	if err != nil {
		return "", err
	}

	id.mu.Lock()
	id.cache[assigner] = cachedToken{token: tok, exp: exp}
	id.mu.Unlock()

	return tok, nil
}

type mintRequest struct {
	GitFlameUsername string `json:"gitflame_username"`
}

type mintResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
}

func (id *Identity) mint(ctx context.Context, assigner string) (token string, exp time.Time, err error) {
	body, err := json.Marshal(mintRequest{GitFlameUsername: assigner})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("identity tokensource: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, id.identityURL+"/identity/v1/bot-token", bytes.NewReader(body))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("identity tokensource: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", id.serviceSecret)

	resp, err := id.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("identity tokensource: request: %w: %w", domain.ErrTransient, err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		var mr mintResponse
		if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
			return "", time.Time{}, fmt.Errorf("identity tokensource: decode: %w: %w", domain.ErrTransient, err)
		}

		// Cache until 30 s before the token actually expires.
		expiry := time.Now().Add(time.Duration(mr.ExpiresIn)*time.Second - 30*time.Second)

		return mr.AccessToken, expiry, nil

	case resp.StatusCode == http.StatusNotFound:
		return "", time.Time{}, domain.ErrNotOnboarded

	case resp.StatusCode >= 500:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", time.Time{}, fmt.Errorf("identity tokensource: status %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrTransient)

	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", time.Time{}, fmt.Errorf("identity tokensource: status %d: %s: %w",
			resp.StatusCode, strings.TrimSpace(string(snippet)), domain.ErrPermanent)
	}
}
