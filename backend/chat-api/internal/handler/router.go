package handler

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

// RegisterRoutes mounts all API routes and middleware onto the given router.
func RegisterRoutes(r chi.Router, chatH *ChatHandler, msgH *MessageHandler, streamH *StreamHandler) {
    r.Use(middleware.Logger)
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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
        r.Get("/chats", chatH.List)
        r.Get("/chats/{chat_id}/messages", msgH.ListByChat)
        r.Post("/chats/{chat_id}/stream", streamH.Stream)
    })
}
