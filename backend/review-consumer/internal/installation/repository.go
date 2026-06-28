package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository looks up installation rows by GitFlame username.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetUserID returns the inno-agent user_id for the given GitFlame username.
// Returns ("", false, nil) if no installation is found.
func (r *Repository) GetUserID(ctx context.Context, gitflameUsername string) (string, bool, error) {
	var userID string
	err := r.pool.QueryRow(
		ctx,
		`SELECT user_id FROM installations WHERE gitflame_username = $1`,
		gitflameUsername,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("installation: get user_id: %w", err)
	}
	return userID, true, nil
}
