package botprincipal

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrUsernameTaken is returned by UpsertConsent when the gitflame_username is
// already linked to a different user_id (first-wins, UNIQUE constraint).
var ErrUsernameTaken = errors.New("gitflame username already linked to another user")

// Repository handles persistence of bot principals (onboarded users who
// consented to let the bot act on their behalf).
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// UpsertConsent links userID to gitflameUsername, or updates the link if it
// already exists for the same user_id.  If another user_id already owns
// gitflameUsername it returns ErrUsernameTaken.
func (r *Repository) UpsertConsent(ctx context.Context, userID, gitflameUsername string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bot_principals (user_id, gitflame_username)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		    SET gitflame_username = EXCLUDED.gitflame_username,
		        consented_at      = now()
	`, userID, gitflameUsername)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// The UNIQUE constraint on gitflame_username fired — another user
			// already owns this username.
			return ErrUsernameTaken
		}

		return fmt.Errorf("upsert consent: %w", err)
	}

	return nil
}

// FindUserIDByGitFlameUsername looks up the user_id for a given
// gitflame_username.  found is false when no row exists (not an error).
func (r *Repository) FindUserIDByGitFlameUsername(ctx context.Context, gitflameUsername string) (userID string, found bool, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT user_id FROM bot_principals WHERE gitflame_username = $1
	`, gitflameUsername).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("find user by gitflame username: %w", err)
	}

	return userID, true, nil
}
