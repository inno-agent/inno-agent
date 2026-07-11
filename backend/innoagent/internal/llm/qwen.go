package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

const (
	defaultHTTPTimeout = 120 * time.Second

	chatCompletionsPath = "/chat/completions"
)

type QwenProvider struct {
	baseURL     string
	model       string
	temperature float64
	apiKey      string

	httpClient *http.Client
}

type QwenOption func(*QwenProvider)

func WithModel(model string) QwenOption {
	return func(p *QwenProvider) {
		p.model = model
	}
}

func WithTemperature(t float64) QwenOption {
	return func(p *QwenProvider) {
		p.temperature = t
	}
}

func WithAPIKey(apiKey string) QwenOption {
	return func(p *QwenProvider) {
		p.apiKey = apiKey
	}
}

func WithHTTPClient(c *http.Client) QwenOption {
	return func(p *QwenProvider) {
		p.httpClient = c
	}
}

func NewQwenProvider(
	baseURL string,
	opts ...QwenOption,
) *QwenProvider {
	p := &QwenProvider{
		baseURL:     strings.TrimRight(baseURL, "/"),
		temperature: 0.7,

		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *QwenProvider) Chat(
	ctx context.Context,
	messages []Message,
	modelName string,
) (string, error) {
	if len(messages) == 0 {
		return "", ErrEmptyMessage
	}

	chatMessages := make([]ChatMessage, len(messages))
	for i, m := range messages {
		chatMessages[i] = ChatMessage(m)
	}

	model := modelName
	if model == "" {
		model = p.model
	}

	reqBody := ChatRequest{
		Model:       model,
		Messages:    chatMessages,
		Temperature: p.temperature,
		MaxTokens:   1024,
		Stream:      false,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf(
			"llm: failed to marshal request: %w",
			err,
		)
	}

	endpoint := p.baseURL + chatCompletionsPath

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", fmt.Errorf(
			"llm: failed to build request: %w",
			err,
		)
	}

	httpReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	httpReq.Header.Set(
		"Accept",
		"application/json",
	)
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	logger.SetCorrelationIDHeader(ctx, httpReq)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf(
			"llm: request failed: %w",
			err,
		)
	}

	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf(
			"llm: failed reading response: %w",
			err,
		)
	}

	if httpResp.StatusCode != http.StatusOK {
		return "", p.parseErrorResponse(
			httpResp.StatusCode,
			body,
		)
	}

	var chatResp ChatResponse

	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf(
			"llm: decode response: %w",
			err,
		)
	}

	if len(chatResp.Choices) == 0 {
		return "", ErrEmptyResponse
	}

	return strings.TrimSpace(
		chatResp.Choices[0].Message.Content,
	), nil
}

func (p *QwenProvider) Stream(ctx context.Context, messages []Message, modelName string) (<-chan string, error) {
	if len(messages) == 0 {
		return nil, ErrEmptyMessage
	}

	chatMessages := make([]ChatMessage, len(messages))
	for i, m := range messages {
		chatMessages[i] = ChatMessage(m)
	}

	model := modelName
	if model == "" {
		model = p.model
	}

	reqBody := ChatRequest{
		Model:       model,
		Messages:    chatMessages,
		Temperature: p.temperature,
		MaxTokens:   2048,
		Stream:      true,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+chatCompletionsPath, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	logger.SetCorrelationIDHeader(ctx, httpReq)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		return nil, p.parseErrorResponse(httpResp.StatusCode, body)
	}

	ch := make(chan string, 4)
	go func() {
		defer func() { _ = httpResp.Body.Close() }()
		defer close(ch)

		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var openaiChunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &openaiChunk); err != nil {
				continue
			}
			if len(openaiChunk.Choices) > 0 && openaiChunk.Choices[0].Delta.Content != "" {
				select {
				case <-ctx.Done():
					return
				case ch <- openaiChunk.Choices[0].Delta.Content:
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return
		}
	}()

	return ch, nil
}

func (p *QwenProvider) parseErrorResponse(
	statusCode int,
	body []byte,
) *ProviderError {
	var errResp ErrorResponse

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Error.Message != "" {
			return newProviderError(
				statusCode,
				errResp.Error.Message,
			)
		}
	}

	raw := string(body)

	if len(raw) > 256 {
		raw = raw[:256] + "..."
	}

	return newProviderError(
		statusCode,
		raw,
	)
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
}

type Choice struct {
	Message ChatMessage `json:"message"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}
