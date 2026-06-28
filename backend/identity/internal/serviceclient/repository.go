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
