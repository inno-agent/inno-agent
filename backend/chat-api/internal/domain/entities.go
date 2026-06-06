package domain

import (
	"time"

	"github.com/google/uuid"
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
	Role      string
	Content   string
	CreatedAt time.Time
}
