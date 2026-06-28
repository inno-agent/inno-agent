package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/transport"
)

type OrchestratorClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewOrchestratorClient(baseURL string) *OrchestratorClient {
	return &OrchestratorClient{
		baseURL:    baseURL,
		httpClient: transport.NewClient(180 * time.Second),
	}
}

func (c *OrchestratorClient) Chat(ctx context.Context, messages []Message, modelName string) (string, error) {
	payload := map[string]interface{}{
		"messages": messages,
	}
	if modelName != "" {
		payload["model_name"] = modelName
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("llm client: marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := middleware.TokenFromContext(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm client: request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

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

type Message = domain.LLMMessage

func (c *OrchestratorClient) Stream(ctx context.Context, messages []Message, modelName string) (<-chan string, error) {
	payload := map[string]interface{}{
		"messages": messages,
		"stream":   true,
	}
	if modelName != "" {
		payload["model_name"] = modelName
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("llm client: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tok := middleware.TokenFromContext(ctx); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm client: request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("llm client: status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" && !strings.Contains(contentType, "text/event-stream") {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("llm client: invalid content-type %s", contentType)
	}

	ch := make(chan string, 64)
	go func() {
		defer func() {
			_ = resp.Body.Close()
		}()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk struct {
				Answer string `json:"answer"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if chunk.Answer != "" {
				select {
				case <-ctx.Done():
					return
				case ch <- chunk.Answer:
				}
			}
		}

		_ = scanner.Err()
	}()

	return ch, nil
}
