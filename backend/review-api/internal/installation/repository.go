// Package installation manages the mapping of gitflame_username → user_id.
package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrOwnedByAnother is returned when a different user_id already holds the gitflame_username.
var ErrOwnedByAnother = errors.New("gitflame username already linked to another user")

// ErrNotLinked is returned when the user has no linked gitflame_username.
var ErrNotLinked = errors.New("no gitflame username linked for user")

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

// GetGitFlameUsername returns the gitflame_username linked to userID.
// Returns ErrNotLinked if the user has not linked an account.
func (r *Repository) GetGitFlameUsername(ctx context.Context, userID string) (string, error) {
	var username string
	err := r.pool.QueryRow(ctx, `
		SELECT gitflame_username FROM installations WHERE user_id = $1
	`, userID).Scan(&username)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotLinked
	}
	if err != nil {
		return "", fmt.Errorf("installation: get gitflame username: %w", err)
	}
	return username, nil
}
