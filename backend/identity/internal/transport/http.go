package transport

import (
	"context"
	"errors"
	"net/http"
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

// OIDCEndpoints describes the public IdP coordinates handed to the browser.
type OIDCEndpoints struct {
	// Authority is the public issuer URL; the browser discovers authorize/token/jwks from it.
	Authority string
	ClientID  string
}

func RegisterHTTPRoutes(r *gin.Engine, p provider.AuthProvider, svc ExchangeServicer, iss *issuer.Issuer, expiry time.Duration, oidc OIDCEndpoints) {
	r.GET("/identity/v1/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authority": oidc.Authority,
			"client_id": oidc.ClientID,
		})
	})

	r.GET("/identity/v1/jwks", func(c *gin.Context) {
		c.JSON(http.StatusOK, iss.PublicKeyJWKS())
	})

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
