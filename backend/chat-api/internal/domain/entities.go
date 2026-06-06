package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Chat struct {
	ID          uuid.UUID
	UserID      string
	Title       *string
	LastMessage string
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

type Message struct {
	ID        uuid.UUID
	UserID    string
	ChatID    uuid.UUID
	Role      Role
	Content   string
	CreatedAt time.Time
}
