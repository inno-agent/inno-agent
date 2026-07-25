package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	TokenKey  contextKey = "auth_token"
)

type validateResponse struct {
	UserID string `json:"user_id"`
}

// Auth validates the Bearer token against the auth service and injects user_id into context.
func Auth(authServiceURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if len(token) > 7 && strings.EqualFold(token[:7], "Bearer ") {
				token = token[7:]
			} else {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			body, _ := json.Marshal(map[string]string{"token": token})
			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, authServiceURL+"/identity/v1/validate", bytes.NewReader(body))
			if err != nil {
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			tracing.PropagateOutbound(r.Context(), req)

			resp, err := http.DefaultClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			var vr validateResponse
			if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil || vr.UserID == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, vr.UserID)
			ctx = context.WithValue(ctx, TokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts user_id from context set by Auth middleware.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

// TokenFromContext extracts the raw bearer token set by Auth middleware.
func TokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(TokenKey).(string)
	return v
}
