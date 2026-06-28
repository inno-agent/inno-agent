package transport

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/refresh"
	"github.com/inno-agent/identity/internal/user"
)

// ExchangeServicer is the subset of user.Service used by the HTTP handler.
type ExchangeServicer interface {
	UpsertIdentity(ctx context.Context, provider, sub, email string) (user.User, error)
}

// RefreshStore is the minimal interface the refresh endpoints need.
// *refresh.Repository satisfies it.
type RefreshStore interface {
	Store(ctx context.Context, userID string, hash []byte, expiresAt time.Time) error
	Lookup(ctx context.Context, hash []byte) (refresh.Row, error)
	Rotate(ctx context.Context, oldHash, newHash []byte, newExpiresAt time.Time, userID string) (string, error)
	Revoke(ctx context.Context, hash []byte) error
	RevokeChainFromID(ctx context.Context, startID string) error
}

// OIDCEndpoints describes the public IdP coordinates handed to the browser.
type OIDCEndpoints struct {
	// Authority is the public issuer URL; the browser discovers authorize/token/jwks from it.
	Authority string
	ClientID  string
}

func RegisterHTTPRoutes(
	r *gin.Engine,
	p provider.AuthProvider,
	svc ExchangeServicer,
	iss *issuer.Issuer,
	expiry time.Duration,
	oidc OIDCEndpoints,
	refreshRepo RefreshStore,
	refreshExpiry time.Duration,
) {
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
			"user_id": claims.UserID,
		})
	})

	r.POST("/identity/v1/exchange", exchangeHandler(p, svc, iss, expiry, refreshRepo, refreshExpiry))
	r.POST("/identity/v1/refresh", refreshHandler(iss, expiry, refreshRepo, refreshExpiry))
	r.POST("/identity/v1/revoke", revokeHandler(refreshRepo))
}

func exchangeHandler(
	p provider.AuthProvider,
	svc ExchangeServicer,
	iss *issuer.Issuer,
	expiry time.Duration,
	refreshRepo RefreshStore,
	refreshExpiry time.Duration,
) gin.HandlerFunc {
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

		token, err := iss.Issue(u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		// Mint and store a refresh token.
		pt, hash, err := refresh.Mint()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		if err := refreshRepo.Store(c.Request.Context(), u.ID, hash, time.Now().Add(refreshExpiry)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":       token,
			"expires_in":         int(expiry.Seconds()),
			"refresh_token":      pt,
			"refresh_expires_in": int(refreshExpiry.Seconds()),
		})
	}
}

func refreshHandler(
	iss *issuer.Issuer,
	expiry time.Duration,
	refreshRepo RefreshStore,
	refreshExpiry time.Duration,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		hash := refresh.Hash(req.RefreshToken)
		row, err := refreshRepo.Lookup(c.Request.Context(), hash)
		if err != nil {
			if errors.Is(err, refresh.ErrRevoked) && row.ReplacedBy != nil {
				// Reuse of a rotated token: revoke the whole descendant chain.
				_ = refreshRepo.RevokeChainFromID(c.Request.Context(), *row.ReplacedBy)
			}

			if errors.Is(err, refresh.ErrExpired) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "token_expired"})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_refresh_token"})
			}
			return
		}

		// Issue new access token.
		accessToken, err := iss.Issue(row.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		// Mint new refresh token and rotate.
		newPT, newHash, err := refresh.Mint()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		if _, err := refreshRepo.Rotate(c.Request.Context(), hash, newHash, time.Now().Add(refreshExpiry), row.UserID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":       accessToken,
			"expires_in":         int(expiry.Seconds()),
			"refresh_token":      newPT,
			"refresh_expires_in": int(refreshExpiry.Seconds()),
		})
	}
}

func revokeHandler(refreshRepo RefreshStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		hash := refresh.Hash(req.RefreshToken)
		if err := refreshRepo.Revoke(c.Request.Context(), hash); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}
