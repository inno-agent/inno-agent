package dtos

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