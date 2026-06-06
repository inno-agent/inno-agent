package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

var _ domain.ChatService = (*ChatService)(nil)

type ChatService struct {
	chatRepo    domain.ChatRepository
	messageRepo domain.MessageRepository
	logger      *zap.Logger
}

func NewChatService(
	chatRepo domain.ChatRepository,
	messageRepo domain.MessageRepository,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
		logger:      logger.With(zap.String("layer", "service")),
	}
}

func (s *ChatService) ListChats(ctx context.Context, userID string, limit, offset int) ([]domain.ChatItem, int, error) {
	chats, total, err := s.chatRepo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list chats",
			zap.String("function", "ListChats"),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("ListChats failed: %w", err)
	}

	items := make([]domain.ChatItem, len(chats))
	for i, c := range chats {
		items[i] = domain.ChatItem{
			ID:          c.ID,
			Title:       c.Title,
			LastMessage: c.LastMessage,
			UpdatedAt:   c.UpdatedAt,
		}
	}
	return items, total, nil
}

func (s *ChatService) GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.MessageDTO, int, error) {
	msgs, total, err := s.messageRepo.ListByChat(ctx, userID, chatID, limit, offset)
	if err != nil {
		s.logger.Error("failed to get history",
			zap.String("function", "GetHistory"),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("GetHistory failed: %w", err)
	}

	items := make([]domain.MessageDTO, len(msgs))
	for i, m := range msgs {
		items[i] = domain.MessageDTO{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		}
	}
	return items, total, nil
}

func (s *ChatService) Stream(ctx context.Context, userID string, chatID uuid.UUID, message string) (<-chan string, error) {
	if chatID == uuid.Nil {
		chat, err := s.chatRepo.Create(ctx, userID, nil)
		if err != nil {
			return nil, fmt.Errorf("Stream failed: %w", err)
		}
		chatID = chat.ID
	}

	_, err := s.messageRepo.Create(ctx, userID, chatID, "user", message)
	if err != nil {
		return nil, fmt.Errorf("Stream failed: %w", err)
	}

	_ = s.chatRepo.UpdateTimestamp(ctx, chatID)

	rawCh := make(chan string)
	outCh := make(chan string)

	go func() {
		defer close(rawCh)
		rawCh <- "stream mock response. llm is not connected yet"
		rawCh <- "stream mock response"
		rawCh <- "still stream mock response"
	}()

	go func() {
		defer close(outCh)
		var sb strings.Builder
		for chunk := range rawCh {
			outCh <- chunk
			sb.WriteString(chunk)
		}
		// use a fresh context — the request ctx may already be cancelled when streaming finishes
		saveCtx := context.Background()
		if _, err := s.messageRepo.Create(saveCtx, userID, chatID, "assistant", sb.String()); err != nil {
			s.logger.Error("failed to save assistant message",
				zap.String("function", "Stream"),
				zap.Error(err),
			)
		}
		if err := s.chatRepo.UpdateTimestamp(saveCtx, chatID); err != nil {
			s.logger.Warn("failed to update chat timestamp",
				zap.String("function", "Stream"),
				zap.Error(err),
			)
		}
	}()

	return outCh, nil
}
