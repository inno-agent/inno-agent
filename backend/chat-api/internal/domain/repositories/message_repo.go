package repositories

import (
    "context"

    "github.com/google/uuid"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/entities"
)

type MessageRepository interface {
    Create(ctx context.Context, userID string, chatID uuid.UUID, role, content string) (*entities.Message, error)
    ListByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]entities.Message, int, error)
}