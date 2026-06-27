// Package installation reads and rotates installation rows in the shared
// inno_review database.
package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

var _ tokensource.Store = (*Repository)(nil)

// Repository reads installations and persists rotated refresh tokens.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Get reads the installation row for the given gitflame username, taking a
// FOR UPDATE row lock. The consumer is single-threaded so the lock contention
// is minimal; it primarily protects against concurrent rotation.
func (r *Repository) Get(ctx context.Context, gitflameUsername string) (tokensource.InstallationRow, bool, error) {
	var row tokensource.InstallationRow

	err := r.pool.QueryRow(ctx, `
		SELECT gitflame_username, refresh_ciphertext, refresh_nonce
		FROM installations
		WHERE gitflame_username = $1
		FOR UPDATE
	`, gitflameUsername).Scan(&row.GitFlameUsername, &row.RefreshCiphertext, &row.RefreshNonce)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tokensource.InstallationRow{}, false, nil
		}
		return tokensource.InstallationRow{}, false, fmt.Errorf("installation: get: %w", err)
	}

	return row, true, nil
}

// UpdateRefresh persists the rotated encrypted refresh token for the username.
func (r *Repository) UpdateRefresh(ctx context.Context, gitflameUsername string, ciphertext, nonce []byte) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE installations
		SET refresh_ciphertext = $2, refresh_nonce = $3, updated_at = now()
		WHERE gitflame_username = $1
	`, gitflameUsername, ciphertext, nonce)
	if err != nil {
		return fmt.Errorf("installation: update refresh: %w", err)
	}

	return nil
}
