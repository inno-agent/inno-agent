package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

type MessageRepo struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewMessageRepo(pool *pgxpool.Pool, logger *zap.Logger) *MessageRepo {
	return &MessageRepo{
		pool:   pool,
		logger: logger.With(zap.String("component", "message_repo")),
	}
}

const (
	queryCreateMessage = `
        INSERT INTO messages (user_id, chat_id, role, content)
        VALUES ($1, $2, $3, $4)
        RETURNING id, user_id, chat_id, role, content, created_at
    `

	queryListMessagesByChat = `
        SELECT m.id, m.user_id, m.chat_id, m.role, m.content, m.created_at
        FROM messages m
        JOIN chats c ON m.chat_id = c.id
        WHERE m.chat_id = $1 AND c.user_id = $2
        ORDER BY m.created_at ASC
        LIMIT $3 OFFSET $4
    `

	queryCountMessagesByChat = `
        SELECT COUNT(*)
        FROM messages m
        JOIN chats c ON m.chat_id = c.id
        WHERE m.chat_id = $1 AND c.user_id = $2
    `
)

func (r *MessageRepo) Create(ctx context.Context, userID string, chatID uuid.UUID, role domain.Role, content string) (*domain.Message, error) {
	log := r.logger.With(
		zap.String("operation", "Create"),
		zap.String("user_id", userID),
		zap.String("chat_id", chatID.String()),
		zap.String("role", string(role)),
	)

	var m domain.Message
	if err := r.pool.QueryRow(ctx, queryCreateMessage, userID, chatID, role, content).
		Scan(&m.ID, &m.UserID, &m.ChatID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
		log.Error("create message failed", zap.Error(err))
		return nil, fmt.Errorf("create message: %w", err)
	}

	return &m, nil
}

func (r *MessageRepo) ListByChat(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.Message, int, error) {
	log := r.logger.With(
		zap.String("operation", "ListByChat"),
		zap.String("user_id", userID),
		zap.String("chat_id", chatID.String()),
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	var total int
	if err := r.pool.QueryRow(ctx, queryCountMessagesByChat, chatID, userID).Scan(&total); err != nil {
		log.Error("count messages failed", zap.Error(err))
		return nil, 0, fmt.Errorf("list messages: count: %w", err)
	}

	if total == 0 {
		return []domain.Message{}, 0, nil
	}

	rows, err := r.pool.Query(ctx, queryListMessagesByChat, chatID, userID, limit, offset)
	if err != nil {
		log.Error("list messages query failed", zap.Error(err))
		return nil, 0, fmt.Errorf("list messages: query: %w", err)
	}
	defer rows.Close()

	var msgs []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.ID, &m.UserID, &m.ChatID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			log.Error("scan message row failed", zap.Error(err))
			return nil, 0, fmt.Errorf("list messages: scan: %w", err)
		}
		msgs = append(msgs, m)
	}

	if err := rows.Err(); err != nil {
		log.Error("iterate message rows failed", zap.Error(err))
		return nil, 0, fmt.Errorf("list messages: rows: %w", err)
	}

	return msgs, total, nil
}
