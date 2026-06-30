package delegation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository manages delegation grants in the identity DB.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Grant creates or re-activates the grant for (userID, clientID).
func (r *Repository) Grant(ctx context.Context, userID, clientID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO delegation_grants (user_id, client_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, client_id) DO UPDATE
			SET revoked_at = NULL, granted_at = now()
	`, userID, clientID)
	if err != nil {
		return fmt.Errorf("delegation: grant: %w", err)
	}
	return nil
}

// HasValidGrant reports whether an active, non-expired grant exists for (userID, clientID).
func (r *Repository) HasValidGrant(ctx context.Context, userID, clientID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM delegation_grants
			WHERE user_id  = $1
			  AND client_id = $2
			  AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > now())
		)
	`, userID, clientID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("delegation: has_valid_grant: %w", err)
	}
	return exists, nil
}
