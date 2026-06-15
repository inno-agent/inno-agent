package domain

import (
	"context"

	"github.com/google/uuid"
)

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRepository defines persistence operations for chats.
type ChatRepository interface {
	Create(ctx context.Context, userID string, title *string) (*Chat, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]Chat, int, error)
	UpdateTimestamp(ctx context.Context, id uuid.UUID) error
	ExistsForUser(ctx context.Context, chatID uuid.UUID, userID string) (bool, error)
	SoftDelete(ctx context.Context, chatID uuid.UUID, userID string) error
}

// MessageRepository defines persistence operations for messages.
type MessageRepository interface {
	Create(ctx context.Context, userID string, chatID uuid.UUID, role Role, content string) (*Message, error)
	ListByChat(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]Message, int, error)
}

// LLMProvider sends a prompt and returns a response from the language model.
type LLMProvider interface {
	Chat(ctx context.Context, messages []LLMMessage, modelName string) (string, error)
	Stream(ctx context.Context, messages []LLMMessage, modelName string) (<-chan string, error)
}

// ChatService defines the application use cases for chat and messaging.
type ChatService interface {
	ListChats(ctx context.Context, userID string, limit, offset int) ([]ChatItem, int, error)
	GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]MessageDTO, int, error)
	Stream(ctx context.Context, userID string, chatID uuid.UUID, message string, modelName string) (<-chan string, uuid.UUID, error)
	DeleteChat(ctx context.Context, userID string, chatID uuid.UUID) error
}
