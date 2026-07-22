package transport

import (
	"context"
	"errors"
	"net/http"
	"strings"
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

// ServiceClientVerifier checks service client credentials.
// *serviceclient.Repository satisfies it.
type ServiceClientVerifier interface {
	Verify(ctx context.Context, clientID, secret string) error
}

// DelegationStore persists and checks delegation grants.
// *delegation.Repository satisfies it.
type DelegationStore interface {
	Grant(ctx context.Context, userID, clientID string) error
	HasValidGrant(ctx context.Context, userID, clientID string) (bool, error)
	Revoke(ctx context.Context, userID, clientID string) error
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
	svcVerifier ServiceClientVerifier,
	serviceTokenExpiry time.Duration,
	delegations DelegationStore,
	delegateExpiry time.Duration,
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
	r.POST("/identity/v1/service-token", serviceTokenHandler(iss, svcVerifier, serviceTokenExpiry))
	r.POST("/identity/v1/delegation-grant", delegationGrantHandler(iss, delegations))
	r.POST("/identity/v1/delegation-revoke", delegationRevokeHandler(iss, delegations))
	r.POST("/identity/v1/token", tokenExchangeHandler(iss, delegations, delegateExpiry))
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

func serviceTokenHandler(
	iss *issuer.Issuer,
	svcVerifier ServiceClientVerifier,
	expiry time.Duration,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ClientID     string `json:"client_id" binding:"required"`
			ClientSecret string `json:"client_secret" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		if err := svcVerifier.Verify(c.Request.Context(), req.ClientID, req.ClientSecret); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
			return
		}

		tok, err := iss.IssueService(req.ClientID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token": tok,
			"expires_in":   int(expiry.Seconds()),
		})
	}
}

// delegationGrantHandler creates a standing grant: "client X may act on behalf of this user".
// Auth: user Bearer token (forwarded by review-api on behalf of the installing user).
// Body: {"client_id": "<service that will act on user's behalf>"}
func delegationGrantHandler(iss *issuer.Issuer, delegations DelegationStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_token"})
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := iss.Verify(rawToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		if strings.HasPrefix(claims.UserID, "svc:") {
			c.JSON(http.StatusForbidden, gin.H{"error": "user_token_required"})
			return
		}

		var req struct {
			ClientID string `json:"client_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		if err := delegations.Grant(c.Request.Context(), claims.UserID, req.ClientID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// delegationRevokeHandler revokes a standing grant: "client X may no longer act
// on behalf of this user". Mirrors delegationGrantHandler's auth and request
// shape exactly; only the store call differs.
// Auth: user Bearer token. Body: {"client_id": "<service to revoke>"}
func delegationRevokeHandler(iss *issuer.Issuer, delegations DelegationStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_token"})
			return
		}
		rawToken := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := iss.Verify(rawToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		if strings.HasPrefix(claims.UserID, "svc:") {
			c.JSON(http.StatusForbidden, gin.H{"error": "user_token_required"})
			return
		}

		var req struct {
			ClientID string `json:"client_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		if err := delegations.Revoke(c.Request.Context(), claims.UserID, req.ClientID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// tokenExchangeHandler implements RFC 8693 token exchange.
// actor_token: service JWT (sub = svc:<client_id>)
// subject_token: user UUID — authorised by a delegation grant in the DB.
func tokenExchangeHandler(
	iss *issuer.Issuer,
	delegations DelegationStore,
	delegateExpiry time.Duration,
) gin.HandlerFunc {
	const grantType = "urn:ietf:params:oauth:grant-type:token-exchange"
	return func(c *gin.Context) {
		var req struct {
			GrantType    string `json:"grant_type"    binding:"required"`
			ActorToken   string `json:"actor_token"   binding:"required"`
			SubjectToken string `json:"subject_token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}
		if req.GrantType != grantType {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
			return
		}

		// Validate actor — must be a valid service JWT.
		claims, err := iss.Verify(req.ActorToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_actor"})
			return
		}
		if !strings.HasPrefix(claims.UserID, "svc:") {
			c.JSON(http.StatusForbidden, gin.H{"error": "actor_not_service"})
			return
		}

		// Authorise via delegation grant: (client_id, user_id) must exist and be active.
		actorClientID := strings.TrimPrefix(claims.UserID, "svc:")
		valid, err := delegations.HasValidGrant(c.Request.Context(), req.SubjectToken, actorClientID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}
		if !valid {
			c.JSON(http.StatusForbidden, gin.H{"error": "grant_required"})
			return
		}

		tok, err := iss.IssueDelegate(req.SubjectToken, claims.UserID, delegateExpiry)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{ //nolint:gosec
			"access_token":      tok,
			"issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
			"token_type":        "Bearer",
			"expires_in":        int(delegateExpiry.Seconds()),
		})
	}
}
