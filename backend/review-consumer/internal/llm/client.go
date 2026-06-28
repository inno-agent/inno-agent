package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

type contextKey int

const tokenKey contextKey = 0

func ContextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

func tokenFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tokenKey).(string); ok {
		return v
	}
	return ""
}

const userIDKey contextKey = 1

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func userIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

type OrchestratorClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewOrchestratorClient(baseURL string) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func (c *OrchestratorClient) Chat(ctx context.Context, messages []domain.LLMMessage, modelName string) (string, error) {
	payload := map[string]interface{}{"messages": messages}
	if modelName != "" {
		payload["model_name"] = modelName
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("llm client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := tokenFromContext(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if uid := userIDFromContext(ctx); uid != "" {
		req.Header.Set("X-User-ID", uid)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm client: request: %w: %w", domain.ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := fmt.Sprintf("llm client: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return "", fmt.Errorf("%s: %w", msg, domain.ErrPermanent)
		}
		return "", fmt.Errorf("%s: %w", msg, domain.ErrTransient)
	}

	var result struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm client: decode: %w", err)
	}
	return result.Answer, nil
}
