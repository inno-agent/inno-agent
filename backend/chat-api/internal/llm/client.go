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

func (c *OrchestratorClient) Chat(ctx context.Context, messages []Message) (string, error) {
	payload, err := json.Marshal(map[string]interface{}{"messages": messages})
	if err != nil {
		return "", fmt.Errorf("llm client: marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

func (c *OrchestratorClient) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"messages": messages,
		"stream":   true,
	})
	if err != nil {
		return nil, fmt.Errorf("llm client: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("llm client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

	ch := make(chan string, 4)
	go func() {
		defer func() {
			_ = resp.Body.Close()
		}()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
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

		// Игнорируем ошибку scanner'а
		_ = scanner.Err()
	}()

	return ch, nil
}
