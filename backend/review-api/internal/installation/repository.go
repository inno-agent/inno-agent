// Package installation manages the mapping of gitflame_username → user_id.
package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrOwnedByAnother is returned when a different user_id already holds the gitflame_username.
var ErrOwnedByAnother = errors.New("gitflame username already linked to another user")

// Repository provides persistence for installation rows.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Upsert inserts or updates the installation row for gitflameUsername.
// If the row already exists with a different user_id it returns ErrOwnedByAnother.
func (r *Repository) Upsert(ctx context.Context, gitflameUsername, userID string) error {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO installations (gitflame_username, user_id)
		VALUES ($1, $2)
		ON CONFLICT (gitflame_username) DO UPDATE
			SET updated_at = now()
		WHERE installations.user_id = EXCLUDED.user_id
	`, gitflameUsername, userID)
	if err != nil {
		return fmt.Errorf("installation: upsert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOwnedByAnother
	}
	return nil
}
