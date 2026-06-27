package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

type Request struct {
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Answer      string
	StatusCode  int
	Bytes       int
	Duration    time.Duration
	TTFT        time.Duration
	FirstByteAt time.Time
}

type RequestResult struct {
	Success     bool
	Error       string
	StatusCode  int
	Latency     time.Duration
	TTFT        time.Duration
	Bytes       int
	Chunks      []string
	TotalChunks int
	FirstByteAt time.Time
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) HealthCheck(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func (c *HTTPClient) Chat(ctx context.Context, messages []Message) (*Response, error) {
	payload, err := json.Marshal(Request{
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Bytes:      len(body),
		Duration:   time.Since(start),
	}, nil
}

func (c *HTTPClient) ChatStream(ctx context.Context, messages []Message) (*Response, error) {
	payload, err := json.Marshal(Request{
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Bytes:      len(body),
		Duration:   time.Since(start),
	}, nil
}

func (c *HTTPClient) BaseURL() string {
	return c.baseURL
}
