package models

import (
    "time"
    "github.com/google/uuid"
)

type Chat struct {
    ID          uuid.UUID `json:"id"`
    UserID      string    `json:"-"`
    Title       *string   `json:"title"`
    LastMessage string    `json:"last_message"`
    CreatedAt   time.Time `json:"-"`
    UpdatedAt   time.Time `json:"updated_at"`
}