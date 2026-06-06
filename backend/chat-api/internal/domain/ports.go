package domain

import (
	"context"

	"github.com/google/uuid"
)

type ChatRepository interface {
	Create(ctx context.Context, userID string, title *string) (*Chat, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]Chat, int, error)
	UpdateTimestamp(ctx context.Context, id uuid.UUID) error
}

type MessageRepository interface {
	Create(ctx context.Context, userID string, chatID uuid.UUID, role, content string) (*Message, error)
	ListByChat(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]Message, int, error)
}

type ChatService interface {
	ListChats(ctx context.Context, userID string, limit, offset int) ([]ChatItem, int, error)
	GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]MessageDTO, int, error)
	Stream(ctx context.Context, userID string, chatID uuid.UUID, message string) (<-chan string, error)
}
