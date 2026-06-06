package transport

import (
	"context"
	"errors"
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

func RegisterHTTPRoutes(r *gin.Engine, p provider.AuthProvider, svc ExchangeServicer, iss *issuer.Issuer, expiry time.Duration, authority, clientID string) {
	r.GET("/auth/v1/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authority": authority,
			"client_id": clientID,
		})
	})
	r.POST("/auth/v1/exchange", exchangeHandler(p, svc, iss, expiry))
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
