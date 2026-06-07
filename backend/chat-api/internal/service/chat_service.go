package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

var _ domain.ChatService = (*ChatService)(nil)

// ChatService handles chat and message business logic.
type ChatService struct {
	chatRepo    domain.ChatRepository
	messageRepo domain.MessageRepository
	llm         domain.LLMProvider
	logger      *zap.Logger
}

// NewChatService creates a ChatService with the given repositories and logger.
func NewChatService(
	chatRepo domain.ChatRepository,
	messageRepo domain.MessageRepository,
	llm domain.LLMProvider,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
		llm:         llm,
		logger:      logger.With(zap.String("layer", "service")),
	}
}

// ListChats returns a paginated list of chats belonging to the given user.
func (s *ChatService) ListChats(ctx context.Context, userID string, limit, offset int) ([]domain.ChatItem, int, error) {
	chats, total, err := s.chatRepo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error(
			"failed to list chats",
			zap.String("function", "ListChats"),
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("ListChats failed: %w", err)
	}

	items := make([]domain.ChatItem, len(chats))
	for i, c := range chats {
		title := ""
		if c.Title != nil {
			title = *c.Title
		}
		items[i] = domain.ChatItem{
			ID:          c.ID,
			Title:       title,
			LastMessage: c.LastMessage,
			UpdatedAt:   c.UpdatedAt,
		}
	}
	return items, total, nil
}

// GetHistory returns paginated message history for the given chat, scoped to the user.
func (s *ChatService) GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.MessageDTO, int, error) {
	msgs, total, err := s.messageRepo.ListByChat(ctx, userID, chatID, limit, offset)
	if err != nil {
		s.logger.Error(
			"failed to get history",
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

// Stream sends a user message and returns a channel of LLM response chunks along with the resolved chat ID.
func (s *ChatService) Stream(ctx context.Context, userID string, chatID uuid.UUID, message string) (<-chan string, uuid.UUID, error) {
	if chatID == uuid.Nil {
		chat, err := s.chatRepo.Create(ctx, userID, nil)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("Stream failed: %w", err)
		}
		chatID = chat.ID
	} else {
		ok, err := s.chatRepo.ExistsForUser(ctx, chatID, userID)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("Stream: check ownership: %w", err)
		}
		if !ok {
			return nil, uuid.Nil, fmt.Errorf("Stream: %w", domain.ErrAccessDenied)
		}
	}

	_, err := s.messageRepo.Create(ctx, userID, chatID, domain.RoleUser, message)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("Stream failed: %w", err)
	}

	if err := s.chatRepo.UpdateTimestamp(ctx, chatID); err != nil {
		s.logger.Warn("failed to update chat timestamp after user message",
			zap.String("function", "Stream"), zap.Error(err))
	}

	rawCh := make(chan string, 4)
	outCh := make(chan string, 4)

	go func() {
		defer close(rawCh)
		answer, err := s.llm.Chat(ctx, message)
		if err != nil {
			s.logger.Error("llm error", zap.String("function", "Stream"), zap.Error(err))
			return
		}
		select {
		case <-ctx.Done():
		case rawCh <- answer:
		}
	}()

	//nolint:gosec // context.Background intentional: save must complete even if request context is cancelled
	go func() {
		defer close(outCh)
		var sb strings.Builder
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-rawCh:
				if !ok {
					// rawCh closed: natural completion or goroutine1 exited via ctx.Done().
					// Both cases make this select branch ready simultaneously — check explicitly.
					if ctx.Err() != nil {
						return
					}
					saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer saveCancel()
					if _, err := s.messageRepo.Create(saveCtx, userID, chatID, domain.RoleAssistant, sb.String()); err != nil {
						s.logger.Error(
							"failed to save assistant message",
							zap.String("function", "Stream"),
							zap.Error(err),
						)
					}
					if err := s.chatRepo.UpdateTimestamp(saveCtx, chatID); err != nil {
						s.logger.Warn(
							"failed to update chat timestamp",
							zap.String("function", "Stream"),
							zap.Error(err),
						)
					}
					return
				}
				select {
				case <-ctx.Done():
					return
				case outCh <- chunk:
					sb.WriteString(chunk)
				}
			}
		}
	}()

	return outCh, chatID, nil
}
