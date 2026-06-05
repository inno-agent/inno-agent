package models

import (
    "time"
    "github.com/google/uuid"
)

type Chat struct {
    ID        uuid.UUID `json:"id"`
    UserID    string    `json:"user_id"`
    Title     *string   `json:"title"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}