package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertIdentity(ctx context.Context, prov, sub, email string) (User, error) {
	// Fast path: look up existing identity
	u, err := r.findByIdentity(ctx, r.pool, prov, sub)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("find identity: %w", err)
	}

	// Slow path: create user + identity in a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Re-check inside transaction to handle concurrent inserts
	u, err = r.findByIdentity(ctx, tx, prov, sub)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("find identity in tx: %w", err)
	}

	err = tx.QueryRow(
		ctx,
		`INSERT INTO users DEFAULT VALUES RETURNING id, created_at, updated_at`,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO user_identities (user_id, provider, sub, email) VALUES ($1,$2,$3,$4)`,
		u.ID, prov, sub, email,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Concurrent insert won the race — discard this tx, use their user.
			_ = tx.Rollback(ctx)
			return r.findByIdentity(ctx, r.pool, prov, sub)
		}
		return User{}, fmt.Errorf("insert identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repository) findByIdentity(ctx context.Context, q querier, prov, sub string) (User, error) {
	var u User
	err := q.QueryRow(ctx, `
		SELECT u.id, u.created_at, u.updated_at
		FROM users u
		JOIN user_identities ui ON ui.user_id = u.id
		WHERE ui.provider = $1 AND ui.sub = $2
	`, prov, sub).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
