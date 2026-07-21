package llm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
)

const (
	defaultMaxResponseBytes = 10 << 20
)

// ErrResponseTooLarge is returned when the runtime's response exceeds the
// configured cap. A truncated body must fail loudly rather than reach the
// client as malformed JSON.
var ErrResponseTooLarge = errors.New("llm: response body too large")

// CompletionsResult is a raw upstream response.
type CompletionsResult struct {
	Status      int
	Body        []byte
	ContentType string
}

// CompletionsClient forwards an OpenAI-shaped chat-completions request to the
// LLM runtime without inspecting it. Keeping it here, next to QwenProvider,
// keeps knowledge of where the runtime lives in a single package.
type CompletionsClient struct {
	baseURL      string
	apiKey       string
	maxRespBytes int64
	httpClient   *http.Client
}

func NewCompletionsClient(baseURL, apiKey string, timeout time.Duration, maxRespBytes int64) *CompletionsClient {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	if maxRespBytes <= 0 {
		maxRespBytes = defaultMaxResponseBytes
	}
	return &CompletionsClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		maxRespBytes: maxRespBytes,
		httpClient:   &http.Client{Timeout: timeout},
	}
}

// Complete POSTs body to <baseURL>/chat/completions and returns the raw
// response. Non-2xx upstream statuses are returned in the result, not as an
// error; only transport failures produce an error.
//
// Complete is non-streaming: it buffers the entire response before returning.
// Accept: application/json is sent unconditionally. Rejecting stream: true
// in request bodies is the caller's responsibility.
func (c *CompletionsClient) Complete(ctx context.Context, body []byte) (*CompletionsResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+chatCompletionsPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: build completions request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	tracing.PropagateOutbound(ctx, req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: completions request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read one byte past the cap so a body sitting exactly on the limit is
	// accepted while a longer one is detectable.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxRespBytes+1))
	if err != nil {
		return nil, fmt.Errorf("llm: read completions response: %w", err)
	}
	if int64(len(respBody)) > c.maxRespBytes {
		return nil, fmt.Errorf("%w: cap %d bytes", ErrResponseTooLarge, c.maxRespBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}

	return &CompletionsResult{
		Status:      resp.StatusCode,
		Body:        respBody,
		ContentType: contentType,
	}, nil
}
