package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role identifies the author of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Chat is the aggregate root for a conversation thread.
type Chat struct {
	ID          uuid.UUID
	UserID      string
	Title       *string
	LastMessage string
	UpdatedAt   time.Time
	CreatedAt   time.Time
	DeletedAt   *time.Time
}

// Message represents a single turn in a chat conversation.
type Message struct {
	ID        uuid.UUID
	UserID    string
	ChatID    uuid.UUID
	Role      Role
	Content   string
	CreatedAt time.Time
}
