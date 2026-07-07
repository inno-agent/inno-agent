package installation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/tokensource"
)

var _ tokensource.UserStore = (*Repository)(nil)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

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
