package transport

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/identity/internal/botprincipal"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/user"
)

// ExchangeServicer is the subset of user.Service used by the HTTP handler.
type ExchangeServicer interface {
	UpsertIdentity(ctx context.Context, provider, sub, email string) (user.User, error)
}

// BotPrincipalServicer is the subset of botprincipal.Service used by the HTTP
// handler.
type BotPrincipalServicer interface {
	UpsertConsent(ctx context.Context, userID, gitflameUsername string) error
	FindUserIDByGitFlameUsername(ctx context.Context, gitflameUsername string) (userID string, found bool, err error)
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
	botSvc BotPrincipalServicer,
	botTokenSecret string,
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

	r.POST("/identity/v1/exchange", exchangeHandler(p, svc, iss, expiry))
	r.POST("/identity/v1/bot/consent", consentHandler(iss, botSvc))
	r.POST("/identity/v1/bot-token", botTokenHandler(iss, botSvc, botTokenSecret, expiry))
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

		token, err := iss.Issue(u.ID)
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

// consentHandler — PUBLIC (browser calls it, bearer-authed as the logged-in
// user). Verifies the aicore token, then upserts the botprincipal row.
//
//	POST /identity/v1/bot/consent
//	Authorization: Bearer <aicore-token>
//	{"gitflame_username": "alice"}
func consentHandler(iss *issuer.Issuer, botSvc BotPrincipalServicer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Auth: verify aicore bearer token.
		authHeader := c.GetHeader("Authorization")
		tokenStr, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok || tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing_token"})
			return
		}

		claims, err := iss.Verify(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}

		var req struct {
			GitFlameUsername string `json:"gitflame_username"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		req.GitFlameUsername = strings.TrimSpace(req.GitFlameUsername)
		if req.GitFlameUsername == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "gitflame_username is required"})
			return
		}

		if err := botSvc.UpsertConsent(c.Request.Context(), claims.UserID, req.GitFlameUsername); err != nil {
			if err == botprincipal.ErrUsernameTaken {
				c.JSON(http.StatusConflict, gin.H{"error": "username_taken"})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// botTokenHandler — INTERNAL ONLY (never exposed through the ingress; nginx
// blocks it at the edge). Mints a short-TTL aicore token for the user who
// linked the given gitflame_username.
//
//	POST /identity/v1/bot-token
//	X-Service-Secret: <secret>
//	{"gitflame_username": "alice"}
func botTokenHandler(iss *issuer.Issuer, botSvc BotPrincipalServicer, secret string, expiry time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Auth: constant-time compare of service secret.
		// If secret is empty OR header is missing/wrong → 401.
		provided := c.GetHeader("X-Service-Secret")
		if secret == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req struct {
			GitFlameUsername string `json:"gitflame_username"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
			return
		}

		req.GitFlameUsername = strings.TrimSpace(req.GitFlameUsername)
		if req.GitFlameUsername == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "gitflame_username is required"})
			return
		}

		userID, found, err := botSvc.FindUserIDByGitFlameUsername(c.Request.Context(), req.GitFlameUsername)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
			return
		}

		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_onboarded"})
			return
		}

		token, err := iss.IssueActor(userID, "innoagent")
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
