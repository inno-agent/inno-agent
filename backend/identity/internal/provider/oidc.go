package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ProviderName identifies the upstream IdP in stored identities.
const ProviderName = "authentik"

type OIDCProvider struct {
	jwks     keyfunc.Keyfunc
	clientID string
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

func NewOIDCProvider(ctx context.Context, issuer, jwksURL, clientID string) (*OIDCProvider, error) {
	issuerHost := issuer
	if u, err := url.Parse(issuer); err == nil && u.Host != "" {
		issuerHost = u.Host
	}
	override := keyfunc.Override{
		Client: &http.Client{
			Transport: &hostRoundTripper{host: issuerHost, rt: http.DefaultTransport},
		},
	}
	jwks, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, override)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS from %s: %w", jwksURL, err)
	}
	return &OIDCProvider{jwks: jwks, clientID: clientID}, nil
}

func (p *OIDCProvider) Validate(_ context.Context, tokenStr string) (ExternalIdentity, error) {
	opts := []jwt.ParserOption{
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	}
	if p.clientID != "" {
		opts = append(opts, jwt.WithAudience(p.clientID))
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
