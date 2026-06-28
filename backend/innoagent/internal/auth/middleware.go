package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

// Middleware extracts the Bearer token, validates it against identity (authN),
// and injects the user_id into the request context.
func Middleware(client *Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := Bearer(r)
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			userID, err := client.Validate(r.Context(), token)
			if err != nil {
				if errors.Is(err, ErrUnauthorized) {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}
				http.Error(w, `{"error":"identity unavailable"}`, http.StatusBadGateway)
				return
			}
			if strings.HasPrefix(userID, "svc:") {
				userID = r.Header.Get("X-User-ID")
				if userID == "" {
					http.Error(w, `{"error":"missing_user_context"}`, http.StatusBadRequest)
					return
				}
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Bearer extracts the bearer token from the Authorization header, or "".
func Bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return h[7:]
	}
	return ""
}

// UserIDFromContext returns the user_id set by Middleware, or "".
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}
