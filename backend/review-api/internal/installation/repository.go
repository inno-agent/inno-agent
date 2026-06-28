// Package installation manages the mapping of gitflame_username → user_id → encrypted refresh token.
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
func (r *Repository) Upsert(ctx context.Context, gitflameUsername, userID string, ciphertext, nonce []byte) error {
	// Use ON CONFLICT with a WHERE guard to detect ownership conflicts.
	// If the conflict row has a different user_id the UPDATE does not match
	// and we can detect the conflict from the rows-affected count.
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO installations (gitflame_username, user_id, refresh_ciphertext, refresh_nonce, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (gitflame_username) DO UPDATE
			SET refresh_ciphertext = EXCLUDED.refresh_ciphertext,
			    refresh_nonce       = EXCLUDED.refresh_nonce,
			    updated_at          = now()
		WHERE installations.user_id = EXCLUDED.user_id
	`, gitflameUsername, userID, ciphertext, nonce)
	if err != nil {
		return fmt.Errorf("installation: upsert: %w", err)
	}

	// If 0 rows were affected the conflict row exists with a different user_id.
	if tag.RowsAffected() == 0 {
		return ErrOwnedByAnother
	}

	return nil
}
