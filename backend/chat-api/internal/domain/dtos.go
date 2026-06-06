package domain

import (
	"time"

	"github.com/google/uuid"
)

type ChatItem struct {
	ID          uuid.UUID `json:"id"`
	Title       *string   `json:"title"`
	LastMessage string    `json:"last_message"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MessageDTO struct {
	ID        uuid.UUID `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
