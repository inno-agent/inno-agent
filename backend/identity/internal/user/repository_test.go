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
