package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

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

type Message = domain.LLMMessage

func (c *OrchestratorClient) Chat(ctx context.Context, messages []Message, modelName string) (string, error) {
	payloadMap := map[string]interface{}{"messages": messages}
	if modelName != "" {
		payloadMap["model_name"] = modelName
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return "", fmt.Errorf("llm client: marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	logger.PropagateHeaders(ctx, req)
	if tok := middleware.TokenFromContext(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm client: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm client: status %d", resp.StatusCode)
	}

	var result struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm client: decode: %w", err)
	}

	return result.Answer, nil
}
