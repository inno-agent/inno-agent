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
)

const (
	defaultHTTPTimeout = 120 * time.Second

	chatCompletionsPath = "/chat/completions"
)

type QwenProvider struct {
	baseURL string
	model   string

	httpClient *http.Client
}

type QwenOption func(*QwenProvider)

func WithModel(model string) QwenOption {
	return func(p *QwenProvider) {
		p.model = model
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
		baseURL: strings.TrimRight(baseURL, "/"),

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
	message string,
) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", ErrEmptyMessage
	}

	reqBody := ChatRequest{
		Model: p.model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: message,
			},
		},

		Temperature: 0.7,
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
