package handler

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/go-chi/chi/v5"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
)

// RegisterRoutes mounts all API routes and middleware onto the given router.
func RegisterRoutes(r chi.Router, chatH *ChatHandler, msgH *MessageHandler, streamH *StreamHandler, authServiceURL string) {
	r.Use(chimw.Logger)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(authServiceURL))
		r.Get("/chats", chatH.List)
		r.Get("/chats/{chat_id}/messages", msgH.ListByChat)
		r.Post("/chats/{chat_id}/stream", streamH.Stream)
	})
}
