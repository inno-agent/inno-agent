package user_test

import (
	"context"
	"os"
	"testing"

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

func TestRepository_UpsertIdentity_NewUser(t *testing.T) {
	pool := getTestPool(t)
	repo := user.NewRepository(pool)

	ctx := context.Background()
	u, err := repo.UpsertIdentity(ctx, "authentik", "ext-sub-001", "alice@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, u.ID)
	assert.Equal(t, "user", u.Tier)
}

func TestRepository_UpsertIdentity_ExistingUser(t *testing.T) {
	pool := getTestPool(t)
	repo := user.NewRepository(pool)

	ctx := context.Background()
	u1, err := repo.UpsertIdentity(ctx, "authentik", "ext-sub-002", "bob@example.com")
	require.NoError(t, err)

	u2, err := repo.UpsertIdentity(ctx, "authentik", "ext-sub-002", "bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, u1.ID, u2.ID, "same user returned on second call")
}

func TestRepository_GetContext(t *testing.T) {
	pool := getTestPool(t)
	repo := user.NewRepository(pool)

	ctx := context.Background()
	u, err := repo.UpsertIdentity(ctx, "authentik", "ext-sub-003", "carol@example.com")
	require.NoError(t, err)

	uctx, err := repo.GetContext(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.ID, uctx.UserID)
	assert.Equal(t, int32(0), uctx.Version)
}

func TestRepository_UpdateContext(t *testing.T) {
	pool := getTestPool(t)
	repo := user.NewRepository(pool)

	ctx := context.Background()
	u, err := repo.UpsertIdentity(ctx, "authentik", "ext-sub-004", "dave@example.com")
	require.NoError(t, err)

	data := []byte(`{"language":"typescript"}`)
	err = repo.UpdateContext(ctx, u.ID, data)
	require.NoError(t, err)

	uctx, err := repo.GetContext(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), uctx.Version)
	assert.JSONEq(t, `{"language":"typescript"}`, string(uctx.Data))
}

func TestRepository_GetContext_NotFound(t *testing.T) {
	pool := getTestPool(t)
	repo := user.NewRepository(pool)

	_, err := repo.GetContext(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, user.ErrNotFound)
}
