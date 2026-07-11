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
	logger.SetCorrelationIDHeader(ctx, req)
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
	logger.SetCorrelationIDHeader(ctx, req)
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

	scanner := bufio.NewScanner(resp.Body)

	// Peek the first SSE data event synchronously. The orchestrator emits a
	// generation failure as `data: {"error": ...}` before any content, so
	// surface it as a Stream error (the handler turns it into an SSE error
	// event) instead of silently swallowing it into an empty, "done" stream.
	firstData, hasFirst := nextDataEvent(scanner)
	if hasFirst && firstData != "[DONE]" {
		var probe struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(firstData), &probe); err == nil && probe.Error != "" {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("llm client: orchestrator stream error: %s", probe.Error)
		}
	}

	ch := make(chan string, 4)
	go func() {
		defer func() {
			_ = resp.Body.Close()
		}()
		defer close(ch)

		// emit parses one data payload and forwards a non-empty answer.
		// Returns false to stop (on [DONE] or context cancellation).
		emit := func(data string) bool {
			if data == "[DONE]" {
				return false
			}
			var chunk struct {
				Answer string `json:"answer"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				return true
			}
			if chunk.Answer != "" {
				select {
				case <-ctx.Done():
					return false
				case ch <- chunk.Answer:
				}
			}
			return true
		}

		if hasFirst && !emit(firstData) {
			return
		}
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			if !emit(strings.TrimPrefix(line, "data: ")) {
				return
			}
		}

		_ = scanner.Err()
	}()

	return ch, nil
}

// nextDataEvent advances the scanner to the next "data: " line and returns its
// payload. Returns ("", false) at end of stream.
func nextDataEvent(scanner *bufio.Scanner) (string, bool) {
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), true
		}
	}
	return "", false
}
