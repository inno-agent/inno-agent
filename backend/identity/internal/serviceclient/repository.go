package serviceclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid client credentials")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Verify(ctx context.Context, clientID, secret string) error {
	var secretHash []byte
	err := r.pool.QueryRow(
		ctx,
		`SELECT secret_hash FROM service_clients WHERE client_id = $1`,
		clientID,
	).Scan(&secretHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("serviceclient: lookup: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword(secretHash, []byte(secret)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// EnsureClient inserts the service client if it does not already exist.
// Safe to call on every startup — idempotent.
func (r *Repository) EnsureClient(ctx context.Context, clientID, secret, name string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("serviceclient: hash secret: %w", err)
	}
	_, err = r.pool.Exec(
		ctx,
		`INSERT INTO service_clients (client_id, secret_hash, name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (client_id) DO NOTHING`,
		clientID, hash, name,
	)
	if err != nil {
		return fmt.Errorf("serviceclient: ensure: %w", err)
	}
	return nil
}
