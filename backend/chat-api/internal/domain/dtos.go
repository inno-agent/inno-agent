package domain

import (
	"time"

	"github.com/google/uuid"
)

// ChatItem is the read model returned in chat list responses.
type ChatItem struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	LastMessage string    `json:"last_message"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MessageDTO is the read model returned in message list and history responses.
type MessageDTO struct {
	ID        uuid.UUID `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
