package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type ZitadelProvider struct {
	jwks     keyfunc.Keyfunc
	clientID string
}

func NewZitadelProvider(ctx context.Context, issuer, jwksURL, clientID string) (*ZitadelProvider, error) {
	jwks, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS from %s: %w", jwksURL, err)
	}
	return &ZitadelProvider{jwks: jwks, clientID: clientID}, nil
}

func (p *ZitadelProvider) Validate(_ context.Context, tokenStr string) (ExternalIdentity, error) {
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
		Provider: "zitadel",
		Sub:      sub,
		Email:    email,
	}, nil
}
