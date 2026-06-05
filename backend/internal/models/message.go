package models

import (
    "time"
    "github.com/google/uuid"
)

type Message struct {
    ID        uuid.UUID `json:"id"`
    UserID    string    `json:"user_id"`
    ChatID    uuid.UUID `json:"chat_id"`
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}