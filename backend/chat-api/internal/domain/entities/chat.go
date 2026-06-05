package entities

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