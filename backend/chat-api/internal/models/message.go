package models

import (
    "time"
    "github.com/google/uuid"
)

type Message struct {
    ID        uuid.UUID `json:"id"`
    UserID    string    `json:"-"`
    ChatID    uuid.UUID `json:"-"`
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
}