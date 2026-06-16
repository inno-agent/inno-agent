package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type ctxKey string

const resultKey ctxKey = "authz_result"

// Middleware extracts the Bearer token, authorizes it against identity, and
// injects the Result into the request context. The model is not checked here
// (the body is not yet parsed) — handlers that need model authz call
// client.Authorize again with the model, or read the policy from the Result.
func Middleware(client *Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearer(r)
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			res, err := client.Authorize(r.Context(), token, "")
			if err != nil {
				if errors.Is(err, ErrUnauthorized) {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}
				http.Error(w, `{"error":"identity unavailable"}`, http.StatusBadGateway)
				return
			}
			ctx := context.WithValue(r.Context(), resultKey, res)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return h[7:]
	}
	return ""
}

// FromContext returns the authorization result set by Middleware, or nil.
func FromContext(ctx context.Context) *Result {
	v, _ := ctx.Value(resultKey).(*Result)
	return v
}
