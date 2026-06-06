package handler

import "github.com/go-chi/chi/v5"

func RegisterRoutes(r chi.Router, chatH *ChatHandler, msgH *MessageHandler, streamH *StreamHandler) {
    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/chats", chatH.List)
        r.Get("/chats/{chat_id}/messages", msgH.ListByChat)
        r.Get("/chats/{chat_id}/stream", streamH.Stream)
    })
}