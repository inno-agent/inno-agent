package tokensource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/identityclient"
)

var _ domain.TokenSource = (*Installation)(nil)

// InstallationRow is a decrypted-at-rest installation as read by the store.
type InstallationRow struct {
	GitFlameUsername  string
	RefreshCiphertext []byte
	RefreshNonce      []byte
}

// Store reads and updates installation rows. Implementations take a row lock
// (FOR UPDATE) so the read+rotate happens atomically per assigner.
type Store interface {
	// Get reads the installation for the gitflame username. Returns
	// (row, false, nil) when no row exists.
	Get(ctx context.Context, gitflameUsername string) (InstallationRow, bool, error)
	// UpdateRefresh persists the rotated (encrypted) refresh token.
	UpdateRefresh(ctx context.Context, gitflameUsername string, ciphertext, nonce []byte) error
}

// RefreshClient exchanges a refresh token for a fresh access token + rotated refresh.
type RefreshClient interface {
	Refresh(ctx context.Context, refreshToken string) (access string, newRefresh string, accessExpiry time.Time, err error)
}

// Crypter encrypts/decrypts refresh tokens at rest.
type Crypter interface {
	Encrypt(plaintext []byte) (ciphertext []byte, nonce []byte, err error)
	Decrypt(ciphertext, nonce []byte) ([]byte, error)
}

// Installation is a TokenSource backed by the inno_review installations table.
// It reads the assigner's encrypted refresh token, uses identity's generic
// /refresh to obtain a fresh access token (rotating the refresh), persists the
// rotated refresh, and caches the access token until ~30s before expiry.
type Installation struct {
	store   Store
	refresh RefreshClient
	crypter Crypter

	mu    sync.Mutex
	cache map[string]cachedAccess
}

type cachedAccess struct {
	token string
	exp   time.Time
}

// NewInstallation creates an Installation TokenSource.
func NewInstallation(store Store, refresh RefreshClient, crypter Crypter) *Installation {
	return &Installation{
		store:   store,
		refresh: refresh,
		crypter: crypter,
		cache:   make(map[string]cachedAccess),
	}
}

// Token returns a valid access token for ref.Assigner.
func (i *Installation) Token(ctx context.Context, ref domain.PRRef) (string, error) {
	assigner := ref.Assigner
	if assigner == "" {
		return "", fmt.Errorf("token: assigner is empty: %w", domain.ErrPermanent)
	}

	// Fast path: cached access token still fresh.
	i.mu.Lock()
	if ca, ok := i.cache[assigner]; ok && time.Now().Before(ca.exp) {
		tok := ca.token
		i.mu.Unlock()
		return tok, nil
	}
	i.mu.Unlock()

	// Read the installation (FOR UPDATE in the real store).
	row, found, err := i.store.Get(ctx, assigner)
	if err != nil {
		return "", fmt.Errorf("token: get installation: %w", err)
	}
	if !found {
		return "", domain.ErrNotOnboarded
	}

	// Decrypt the stored refresh token.
	plainRefresh, err := i.crypter.Decrypt(row.RefreshCiphertext, row.RefreshNonce)
	if err != nil {
		return "", fmt.Errorf("token: decrypt refresh: %w: %w", domain.ErrPermanent, err)
	}

	// Exchange it via identity's generic /refresh (rotating).
	access, newRefresh, accessExp, err := i.refresh.Refresh(ctx, string(plainRefresh))
	if err != nil {
		if errors.Is(err, identityclient.ErrGrantDead) {
			// The grant is dead — treat as not onboarded (consumer prompts reconnect).
			return "", domain.ErrNotOnboarded
		}
		return "", fmt.Errorf("token: refresh: %w", err)
	}

	// Persist the rotated refresh token (encrypted).
	ct, nonce, err := i.crypter.Encrypt([]byte(newRefresh))
	if err != nil {
		return "", fmt.Errorf("token: encrypt rotated refresh: %w: %w", domain.ErrPermanent, err)
	}
	if err := i.store.UpdateRefresh(ctx, assigner, ct, nonce); err != nil {
		return "", fmt.Errorf("token: persist rotated refresh: %w", err)
	}

	// Cache the access token until ~30s before it actually expires.
	cacheExp := accessExp.Add(-30 * time.Second)
	i.mu.Lock()
	i.cache[assigner] = cachedAccess{token: access, exp: cacheExp}
	i.mu.Unlock()

	return access, nil
}
