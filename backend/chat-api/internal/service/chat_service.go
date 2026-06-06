package service

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/dtos"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/repositories"
    domainServices "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/services"
)

var _ domainServices.Service = (*ChatService)(nil)

type ChatService struct {
    chatRepo    repositories.ChatRepository
    messageRepo repositories.MessageRepository
    logger      *zap.Logger
}

func NewChatService(
    chatRepo repositories.ChatRepository,
    messageRepo repositories.MessageRepository,
    logger *zap.Logger,
) *ChatService {
    return &ChatService{
        chatRepo:    chatRepo,
        messageRepo: messageRepo,
        logger:      logger.With(zap.String("layer", "service")),
    }
}

func (s *ChatService) ListChats(ctx context.Context, userID string, limit, offset int) ([]dtos.ChatItem, int, error) {
    chats, total, err := s.chatRepo.ListByUser(ctx, userID, limit, offset)
    if err != nil {
        s.logger.Error("failed to list chats",
            zap.String("function", "ListChats"),
            zap.Error(err),
        )
        return nil, 0, fmt.Errorf("ListChats failed: %w", err)
    }

    items := make([]dtos.ChatItem, len(chats))
    for i, c := range chats {
        items[i] = dtos.ChatItem{
            ID:          c.ID,
            Title:       c.Title,
            LastMessage: c.LastMessage,
            UpdatedAt:   c.UpdatedAt,
        }
    }
    return items, total, nil
}

func (s *ChatService) GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]dtos.Message, int, error) {
    msgs, total, err := s.messageRepo.ListByChat(ctx, userID, chatID, limit, offset)
    if err != nil {
        s.logger.Error("failed to get history",
            zap.String("function", "GetHistory"),
            zap.Error(err),
        )
        return nil, 0, fmt.Errorf("GetHistory failed: %w", err)
    }

    items := make([]dtos.Message, len(msgs))
    for i, m := range msgs {
        items[i] = dtos.Message{
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

    ch := make(chan string)
    go func() {
        defer close(ch)
        ch <- "stream mock response. llm is not connected yet"
        ch <- "stream mock response"
        ch <- "still stream mock response"
    }()

    go func() {
    var fullResponse string
    for chunk := range ch {
        fullResponse += chunk
    }
    saveCtx := context.Background()
    _, err := s.messageRepo.Create(saveCtx, userID, chatID, "assistant", fullResponse)
    if err != nil {
        s.logger.Error("failed to save assistant message",
            zap.String("function", "Stream"),
            zap.Error(err),
        )
    }
    _ = s.chatRepo.UpdateTimestamp(saveCtx, chatID)
}()

    return ch, nil
}