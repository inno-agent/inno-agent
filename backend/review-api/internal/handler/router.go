package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

// RegisterRoutes mounts all API routes and middleware onto the given router.
func RegisterRoutes(r chi.Router, reviewH *ReviewHandler, authServiceURL string) {
	r.Use(chimw.Logger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1", func(r chi.Router) {
		if authServiceURL != "" {
			r.Use(middleware.Auth(authServiceURL))
		}
		r.Post("/review", reviewH.Review)
	})
}
