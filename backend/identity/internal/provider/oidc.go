package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ProviderName identifies the upstream IdP in stored identities.
const ProviderName = "authentik"

type OIDCProvider struct {
	jwks     keyfunc.Keyfunc
	clientID string
	issuer   string
}

// hostRoundTripper forces the Host header so the IdP builds public-facing URLs
// even when reached via an internal Docker hostname.
type hostRoundTripper struct {
	host string
	rt   http.RoundTripper
}

func (t *hostRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Host = t.host
	return t.rt.RoundTrip(req)
}

func issuerHost(issuer string) string {
	if u, err := url.Parse(issuer); err == nil && u.Host != "" {
		return u.Host
	}
	return issuer
}

func NewOIDCProvider(ctx context.Context, issuer, jwksURL, clientID string) (*OIDCProvider, error) {
	override := keyfunc.Override{
		Client: &http.Client{
			Transport: &hostRoundTripper{host: issuerHost(issuer), rt: http.DefaultTransport},
		},
	}
	jwks, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, override)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS from %s: %w", jwksURL, err)
	}
	return &OIDCProvider{jwks: jwks, clientID: clientID, issuer: issuer}, nil
}

// probeJWKS verifies the endpoint serves a non-empty key set. keyfunc tolerates
// a failed initial fetch and would otherwise silently start with zero keys,
// rejecting every token until its next scheduled refresh.
func probeJWKS(ctx context.Context, jwksURL, host string) error {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &hostRoundTripper{host: host, rt: http.DefaultTransport},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}
	var body struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}
	if len(body.Keys) == 0 {
		return errors.New("jwks endpoint returned no keys")
	}
	return nil
}

// NewOIDCProviderWithRetry waits for the JWKS endpoint to become available
// before constructing the provider. On a fresh deployment the authentik worker
// may not have applied the provider blueprint yet, so the JWKS endpoint 404s
// for a short while after authentik-server is healthy.
func NewOIDCProviderWithRetry(ctx context.Context, issuer, jwksURL, clientID string, attempts int, delay time.Duration) (*OIDCProvider, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		lastErr = probeJWKS(ctx, jwksURL, issuerHost(issuer))
		if lastErr == nil {
			return NewOIDCProvider(ctx, issuer, jwksURL, clientID)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("jwks not ready after %d attempts: %w", attempts, lastErr)
}

func (p *OIDCProvider) Validate(_ context.Context, tokenStr string) (ExternalIdentity, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	}
	if p.clientID != "" {
		opts = append(opts, jwt.WithAudience(p.clientID))
	}
	if p.issuer != "" {
		opts = append(opts, jwt.WithIssuer(p.issuer))
	}
	token, err := jwt.Parse(tokenStr, p.jwks.Keyfunc, opts...)
	if err != nil {
		return ExternalIdentity{}, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return ExternalIdentity{}, errors.New("invalid token claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return ExternalIdentity{}, errors.New("missing sub claim")
	}
	email, _ := claims["email"].(string)

	return ExternalIdentity{
		Provider: ProviderName,
		Sub:      sub,
		Email:    email,
	}, nil
}
