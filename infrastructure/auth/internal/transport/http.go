package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/auth/internal/issuer"
	"github.com/inno-agent/auth/internal/provider"
	"github.com/inno-agent/auth/internal/user"
)

// ExchangeServicer is the subset of user.Service used by the HTTP handler.
type ExchangeServicer interface {
	UpsertIdentity(ctx context.Context, provider, sub, email string) (user.User, error)
	GetContext(ctx context.Context, userID string) (user.UserContext, error)
}

func RegisterHTTPRoutes(r *gin.Engine, p provider.AuthProvider, svc ExchangeServicer, iss *issuer.Issuer, expiry time.Duration, authority, clientID, zitadelBaseURL string) {
	r.GET("/auth/v1/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authority":              authority,
			"client_id":              clientID,
			"authorization_endpoint": authority + "/oauth/v2/authorize",
		})
	})

	r.GET("/auth/v1/jwks", func(c *gin.Context) {
		c.JSON(http.StatusOK, iss.PublicKeyJWKS())
	})

	// Proxy Zitadel OIDC endpoints so browser never makes HTTP requests directly
	// (avoids mixed-content block when app is on HTTPS but Zitadel is on HTTP).
	r.GET("/auth/v1/oidc/jwks", proxyGet(zitadelBaseURL+"/oauth/v2/keys"))
	r.POST("/auth/v1/oidc/token", proxyPost(zitadelBaseURL+"/oauth/v2/token"))

	r.POST("/auth/v1/validate", func(c *gin.Context) {
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

	r.POST("/auth/v1/exchange", exchangeHandler(p, svc, iss, expiry))
}

func proxyGet(target string) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := http.Get(target) //nolint:noctx
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "proxy error"})
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
}

func proxyPost(target string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read error"})
			return
		}
		resp, err := http.Post(target, c.Request.Header.Get("Content-Type"), bytes.NewReader(body)) //nolint:noctx
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "proxy error"})
			return
		}
		defer func() { _ = resp.Body.Close() }()
		respBody, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
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
