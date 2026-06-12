package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/user"
)

// ExchangeServicer is the subset of user.Service used by the HTTP handler.
type ExchangeServicer interface {
	UpsertIdentity(ctx context.Context, provider, sub, email string) (user.User, error)
	GetContext(ctx context.Context, userID string) (user.UserContext, error)
}

// OIDCEndpoints describes the upstream IdP endpoints exposed to and proxied for the browser.
type OIDCEndpoints struct {
	// Authority is the public issuer URL (what the browser validates id_token iss against).
	Authority string
	// AuthorizeURL is the public authorization endpoint the browser is redirected to.
	AuthorizeURL string
	ClientID     string
	// TokenURL is the internal token endpoint, proxied via /identity/v1/oidc/token.
	TokenURL string
	// JWKSURL is the internal JWKS endpoint, proxied via /identity/v1/oidc/jwks.
	JWKSURL string
}

func RegisterHTTPRoutes(r *gin.Engine, p provider.AuthProvider, svc ExchangeServicer, iss *issuer.Issuer, expiry time.Duration, oidc OIDCEndpoints) {
	r.GET("/identity/v1/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authority":              oidc.Authority,
			"client_id":              oidc.ClientID,
			"authorization_endpoint": oidc.AuthorizeURL,
		})
	})

	r.GET("/identity/v1/jwks", func(c *gin.Context) {
		c.JSON(http.StatusOK, iss.PublicKeyJWKS())
	})

	// Proxy IdP OIDC endpoints so the browser only talks to our origin.
	// Host and X-Forwarded-Proto must match the public authority: the IdP
	// builds the id_token issuer from them, and the browser OIDC client
	// rejects tokens whose iss differs from the configured authority.
	authorityHost, authorityScheme := splitAuthority(oidc.Authority)
	r.GET("/identity/v1/oidc/jwks", proxyGet(oidc.JWKSURL, authorityHost, authorityScheme))
	r.POST("/identity/v1/oidc/token", proxyPost(oidc.TokenURL, authorityHost, authorityScheme))

	r.POST("/identity/v1/validate", func(c *gin.Context) {
		var req struct {
			Token string `json:"token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		claims, err := iss.Verify(req.Token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user_id":     claims.UserID,
			"tier":        claims.Tier,
			"ctx_version": claims.CtxVersion,
		})
	})

	r.POST("/identity/v1/exchange", exchangeHandler(p, svc, iss, expiry))
}

// splitAuthority extracts host:port and scheme from an authority URL,
// e.g. "https://localhost:8080/application/o/app/" → ("localhost:8080", "https").
func splitAuthority(authority string) (host, scheme string) {
	if u, err := url.Parse(authority); err == nil && u.Host != "" {
		return u.Host, u.Scheme
	}
	return authority, "https"
}

// proxyClient bounds calls to the IdP so a hung authentik can't pile up goroutines.
var proxyClient = &http.Client{Timeout: 10 * time.Second}

func proxyRequest(c *gin.Context, req *http.Request, host, scheme string) {
	req.Host = host
	req.Header.Set("X-Forwarded-Proto", scheme)
	resp, err := proxyClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "proxy error"})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func proxyGet(target, host, scheme string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target, nil)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "proxy error"})
			return
		}
		proxyRequest(c, req, host, scheme)
	}
}

func proxyPost(target, host, scheme string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read error"})
			return
		}
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "proxy error"})
			return
		}
		req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		proxyRequest(c, req, host, scheme)
	}
}

func exchangeHandler(p provider.AuthProvider, svc ExchangeServicer, iss *issuer.Issuer, expiry time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Token string `json:"token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		identity, err := p.Validate(c.Request.Context(), req.Token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}

		u, err := svc.UpsertIdentity(c.Request.Context(), identity.Provider, identity.Sub, identity.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		uctx, err := svc.GetContext(c.Request.Context(), u.ID)
		if err != nil && !errors.Is(err, user.ErrNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		token, err := iss.Issue(u.ID, u.Tier, uctx.Version)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token": token,
			"expires_in":   int(expiry.Seconds()),
		})
	}
}
