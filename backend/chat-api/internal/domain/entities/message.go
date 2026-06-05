package entities

import (
    "time"
    "github.com/google/uuid"
)

type Message struct {
    ID        uuid.UUID
    UserID    string
    ChatID    uuid.UUID
    Role      string
    Content   string
    CreatedAt time.Time
}