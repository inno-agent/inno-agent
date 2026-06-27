package refresh

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when the token hash has no matching row.
var ErrNotFound = errors.New("refresh token not found")

// ErrRevoked is returned when the token exists but has been revoked.
var ErrRevoked = errors.New("refresh token revoked")

// ErrExpired is returned when the token exists but has expired.
var ErrExpired = errors.New("refresh token expired")

// Row is a row from the refresh_tokens table.
type Row struct {
	ID         string
	UserID     string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
}

// Repository provides refresh-token persistence over a pgx pool.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Mint generates a cryptographically random opaque token and returns its
// base64url plaintext and SHA-256 hash.  The plaintext is returned to the
// caller (sent to the client); only the hash is stored.
func Mint() (plaintext string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("refresh: rand: %w", err)
	}

	pt := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(pt))

	return pt, h[:], nil
}

// Hash returns the SHA-256 hash of a plaintext refresh token.
func Hash(plaintext string) []byte {
	h := sha256.Sum256([]byte(plaintext))
	return h[:]
}

// Store persists a new refresh token row.
func (r *Repository) Store(ctx context.Context, userID string, hash []byte, expiresAt time.Time) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("refresh: store: %w", err)
	}

	return nil
}

// Lookup retrieves the row for the given token hash.
// Returns ErrNotFound if no row exists, ErrExpired if expired,
// ErrRevoked if already revoked.
func (r *Repository) Lookup(ctx context.Context, hash []byte) (Row, error) {
	var row Row
	var replacedByStr *string

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, user_id, expires_at, revoked_at, replaced_by::text
		 FROM refresh_tokens WHERE token_hash = $1`,
		hash,
	).Scan(&row.ID, &row.UserID, &row.ExpiresAt, &row.RevokedAt, &replacedByStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Row{}, ErrNotFound
		}

		return Row{}, fmt.Errorf("refresh: lookup: %w", err)
	}

	row.ReplacedBy = replacedByStr

	if row.RevokedAt != nil {
		return row, ErrRevoked
	}

	if time.Now().After(row.ExpiresAt) {
		return row, ErrExpired
	}

	return row, nil
}

// Rotate atomically revokes the old token and inserts a new one in one tx.
// Returns the new row ID.
func (r *Repository) Rotate(ctx context.Context, oldHash, newHash []byte, newExpiresAt time.Time, userID string) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("refresh: rotate: begin tx: %w", err)
	}

	defer tx.Rollback(ctx) //nolint:errcheck

	// Insert new token first so we have its id.
	var newID string

	err = tx.QueryRow(
		ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
		userID, newHash, newExpiresAt,
	).Scan(&newID)
	if err != nil {
		return "", fmt.Errorf("refresh: rotate: insert new: %w", err)
	}

	// Revoke old token and point it at the new one.
	_, err = tx.Exec(
		ctx,
		`UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $1
		 WHERE token_hash = $2 AND revoked_at IS NULL`,
		newID, oldHash,
	)
	if err != nil {
		return "", fmt.Errorf("refresh: rotate: revoke old: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("refresh: rotate: commit: %w", err)
	}

	return newID, nil
}

// Revoke marks the token and all its descendants as revoked.
// Idempotent — safe to call on an already-revoked token.
func (r *Repository) Revoke(ctx context.Context, hash []byte) error {
	var id string

	err := r.pool.QueryRow(
		ctx,
		`SELECT id FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // nothing to revoke
		}

		return fmt.Errorf("refresh: revoke: lookup id: %w", err)
	}

	return r.RevokeChainFromID(ctx, id)
}

// RevokeChainFromID revokes a whole descendant chain starting from the given
// token id.  Used for reuse detection.
func (r *Repository) RevokeChainFromID(ctx context.Context, startID string) error {
	// Walk the replaced_by chain iteratively to avoid recursive CTE complexity.
	currentID := startID

	for currentID != "" {
		var replacedBy *string

		err := r.pool.QueryRow(
			ctx,
			`UPDATE refresh_tokens SET revoked_at = now()
			 WHERE id = $1 AND revoked_at IS NULL
			 RETURNING replaced_by::text`,
			currentID,
		).Scan(&replacedBy)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Already revoked or doesn't exist — stop.
				break
			}

			return fmt.Errorf("refresh: revoke chain: %w", err)
		}

		if replacedBy == nil {
			break
		}

		currentID = *replacedBy
	}

	return nil
}
