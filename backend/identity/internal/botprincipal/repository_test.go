package botprincipal_test

import (
	"context"
	"os"
	"testing"

	"github.com/inno-agent/identity/internal/botprincipal"
	"github.com/inno-agent/identity/internal/db"
	"github.com/inno-agent/identity/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestPool(t *testing.T) *db.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set — skipping integration test")
	}
	err := db.Migrate(dsn)
	require.NoError(t, err)

	pool, err := db.NewPool(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
	})
	return pool
}

// createUser is a test helper that inserts a real user row and returns its ID.
func createUser(t *testing.T, pool *db.Pool, sub, email string) string {
	t.Helper()
	userRepo := user.NewRepository(pool)
	u, err := userRepo.UpsertIdentity(context.Background(), "authentik", sub, email)
	require.NoError(t, err)
	return u.ID
}

func TestBotPrincipalRepository_UpsertConsent_NewEntry(t *testing.T) {
	pool := getTestPool(t)
	repo := botprincipal.NewRepository(pool)

	userID := createUser(t, pool, "bp-sub-001", "bp1@example.com")

	err := repo.UpsertConsent(context.Background(), userID, "alice-gf")
	require.NoError(t, err)
}

func TestBotPrincipalRepository_UpsertConsent_UpdateExisting(t *testing.T) {
	pool := getTestPool(t)
	repo := botprincipal.NewRepository(pool)

	userID := createUser(t, pool, "bp-sub-002", "bp2@example.com")

	err := repo.UpsertConsent(context.Background(), userID, "bob-gf")
	require.NoError(t, err)

	// Update to a new username for the same user.
	err = repo.UpsertConsent(context.Background(), userID, "bob-gf-new")
	require.NoError(t, err)

	gotID, found, err := repo.FindUserIDByGitFlameUsername(context.Background(), "bob-gf-new")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, userID, gotID)
}

func TestBotPrincipalRepository_UpsertConsent_UsernameTaken(t *testing.T) {
	pool := getTestPool(t)
	repo := botprincipal.NewRepository(pool)

	userID1 := createUser(t, pool, "bp-sub-003", "bp3@example.com")
	userID2 := createUser(t, pool, "bp-sub-004", "bp4@example.com")

	err := repo.UpsertConsent(context.Background(), userID1, "taken-gf")
	require.NoError(t, err)

	// A different user tries to claim the same gitflame username.
	err = repo.UpsertConsent(context.Background(), userID2, "taken-gf")
	require.ErrorIs(t, err, botprincipal.ErrUsernameTaken)
}

func TestBotPrincipalRepository_FindUserIDByGitFlameUsername_Found(t *testing.T) {
	pool := getTestPool(t)
	repo := botprincipal.NewRepository(pool)

	userID := createUser(t, pool, "bp-sub-005", "bp5@example.com")
	err := repo.UpsertConsent(context.Background(), userID, "charlie-gf")
	require.NoError(t, err)

	gotID, found, err := repo.FindUserIDByGitFlameUsername(context.Background(), "charlie-gf")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, userID, gotID)
}

func TestBotPrincipalRepository_FindUserIDByGitFlameUsername_NotFound(t *testing.T) {
	pool := getTestPool(t)
	repo := botprincipal.NewRepository(pool)

	_, found, err := repo.FindUserIDByGitFlameUsername(context.Background(), "no-such-user")
	require.NoError(t, err)
	assert.False(t, found)
}
