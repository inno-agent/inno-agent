package services

import (
    "context"

    "github.com/google/uuid"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/dtos"
)

type Service interface {
    ListChats(ctx context.Context, userID string, limit, offset int) ([]dtos.ChatItem, int, error)
    GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]dtos.Message, int, error)
    Stream(ctx context.Context, userID string, chatID uuid.UUID, message string) (<-chan string, error)
}