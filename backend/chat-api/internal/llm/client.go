package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
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

func (c *OrchestratorClient) Chat(ctx context.Context, messages []Message) (string, error) {
	payload, _ := json.Marshal(map[string]interface{}{"messages": messages})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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
