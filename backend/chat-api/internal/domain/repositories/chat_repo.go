package repositories

import (
    "context"

    "github.com/google/uuid"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/entities"
)

type ChatRepository interface {
    Create(ctx context.Context, userID string, title *string) (*entities.Chat, error)
    ListByUser(ctx context.Context, userID string, limit, offset int) ([]entities.Chat, int, error)
    UpdateTimestamp(ctx context.Context, id uuid.UUID) error
}