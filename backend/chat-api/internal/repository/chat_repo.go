package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

// ChatRepo is the PostgreSQL implementation of domain.ChatRepository.
type ChatRepo struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewChatRepo creates a ChatRepo backed by the given connection pool.
func NewChatRepo(pool *pgxpool.Pool, logger *zap.Logger) *ChatRepo {
	return &ChatRepo{
		pool:   pool,
		logger: logger.With(zap.String("component", "chat_repo")),
	}
}

const (
	queryCreateChat = `
        INSERT INTO chats (user_id, title)
        VALUES ($1, $2)
        RETURNING id, title, updated_at
    `

	queryListChatsByUser = `
        SELECT c.id, c.title, c.updated_at,
               COALESCE(
                   (SELECT content FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1),
                   ''
               ) AS last_message,
               COUNT(*) OVER() AS total
        FROM chats c
        WHERE c.user_id = $1
        ORDER BY c.updated_at DESC
        LIMIT $2 OFFSET $3
    `

	queryUpdateChat = `
        UPDATE chats SET updated_at = now() WHERE id = $1
    `

	queryExistsChatForUser = `SELECT EXISTS(SELECT 1 FROM chats WHERE id = $1 AND user_id = $2)`
)

// Create inserts a new chat row and returns the created Chat.
func (r *ChatRepo) Create(ctx context.Context, userID string, title *string) (*domain.Chat, error) {
	log := r.logger.With(
		zap.String("operation", "Create"),
		zap.String("user_id", userID),
	)

	var c domain.Chat
	if err := r.pool.QueryRow(ctx, queryCreateChat, userID, title).
		Scan(&c.ID, &c.Title, &c.UpdatedAt); err != nil {
		log.Error("create chat failed", zap.Error(err))
		return nil, fmt.Errorf("create chat: %w", err)
	}

	return &c, nil
}

// ListByUser returns a paginated list of chats for the given user along with the total count.
func (r *ChatRepo) ListByUser(ctx context.Context, userID string, limit, offset int) ([]domain.Chat, int, error) {
	log := r.logger.With(
		zap.String("operation", "ListByUser"),
		zap.String("user_id", userID),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	rows, err := r.pool.Query(ctx, queryListChatsByUser, userID, limit, offset)
	if err != nil {
		log.Error("list chats query failed", zap.Error(err))
		return nil, 0, fmt.Errorf("list chats: query: %w", err)
	}
	defer rows.Close()

	var (
		chats []domain.Chat
		total int
	)
	for rows.Next() {
		var c domain.Chat
		if err := rows.Scan(&c.ID, &c.Title, &c.UpdatedAt, &c.LastMessage, &total); err != nil {
			log.Error("scan chat row failed", zap.Error(err))
			return nil, 0, fmt.Errorf("list chats: scan: %w", err)
		}
		chats = append(chats, c)
	}

	if err := rows.Err(); err != nil {
		log.Error("iterate chat rows failed", zap.Error(err))
		return nil, 0, fmt.Errorf("list chats: rows: %w", err)
	}

	if chats == nil {
		return []domain.Chat{}, 0, nil
	}

	return chats, total, nil
}

// ExistsForUser reports whether a chat with the given ID belongs to the given user.
func (r *ChatRepo) ExistsForUser(ctx context.Context, chatID uuid.UUID, userID string) (bool, error) {
	log := r.logger.With(
		zap.String("operation", "ExistsForUser"),
		zap.String("chat_id", chatID.String()),
		zap.String("user_id", userID),
	)

	var exists bool
	if err := r.pool.QueryRow(ctx, queryExistsChatForUser, chatID, userID).Scan(&exists); err != nil {
		log.Error("exists chat for user failed", zap.Error(err))
		return false, fmt.Errorf("exists chat for user: %w", err)
	}

	return exists, nil
}

// UpdateTimestamp sets the updated_at column of a chat to the current time.
func (r *ChatRepo) UpdateTimestamp(ctx context.Context, id uuid.UUID) error {
	log := r.logger.With(
		zap.String("operation", "UpdateTimestamp"),
		zap.String("chat_id", id.String()),
	)

	tag, err := r.pool.Exec(ctx, queryUpdateChat, id)
	if err != nil {
		log.Error("update chat timestamp failed", zap.Error(err))
		return fmt.Errorf("update chat timestamp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update timestamp: chat not found")
	}

	return nil
}
