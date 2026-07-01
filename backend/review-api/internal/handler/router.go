package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

// RegisterRoutes mounts all API routes and middleware onto the given router.
// installH may be nil when the review database is not configured (dev mode); in
// that case the /installations route is not registered.
func RegisterRoutes(r chi.Router, reviewH *ReviewHandler, installH *InstallationHandler, authServiceURL string, logger *zap.Logger) {
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CorrelationID)
	r.Use(middleware.RequestLogger())
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")
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
		if installH != nil {
			r.Post("/installations", installH.Create)
		}
	})
}
