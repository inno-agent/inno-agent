package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestSoftDeleteChat(t *testing.T) {
	pool := setupDB(t)
	logger := zap.NewNop()
	repo := NewChatRepo(pool, logger)

	ctx := context.Background()
	userID := "test-user-delete"
	title := "Chat to delete"

	chat, err := repo.Create(ctx, userID, &title)
	if err != nil {
		t.Fatal(err)
	}

	chats, total, err := repo.ListByUser(ctx, userID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(chats) != 1 {
		t.Fatalf("expected 1 chat, got %d (total %d)", len(chats), total)
	}

	err = repo.SoftDelete(ctx, chat.ID, userID)
	if err != nil {
		t.Fatal(err)
	}

	chats, total, err = repo.ListByUser(ctx, userID, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(chats) != 0 {
		t.Fatalf("expected 0 chats after soft delete, got %d (total %d)", len(chats), total)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM messages WHERE chat_id = $1", chat.ID); err != nil {
		t.Logf("cleanup error: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM chats WHERE id = $1", chat.ID); err != nil {
		t.Logf("cleanup error: %v", err)
	}
}
