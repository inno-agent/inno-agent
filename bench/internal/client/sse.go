package client

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
)

type SSEClient struct {
	baseURL    string
	httpClient *http.Client
}

type SSEEvent struct {
	Type  string
	Data  string
	Raw   string
}

type SSEResult struct {
	Success     bool
	Error       string
	StatusCode  int
	TTFT        time.Duration
	TotalTime   time.Duration
	Bytes       int
	Chunks      []string
	TotalChunks int
	Events      []SSEEvent
	FinishedAt  time.Time
}

func NewSSEClient(baseURL string, timeout time.Duration) *SSEClient {
	return &SSEClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *SSEClient) Stream(ctx context.Context, messages []Message) (*SSEResult, error) {
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &SSEResult{
			StatusCode: resp.StatusCode,
			Success:    false,
			Error:      fmt.Sprintf("status %d: %s", resp.StatusCode, string(body)),
			TotalTime:  time.Since(start),
		}, nil
	}

	result := &SSEResult{
		StatusCode: resp.StatusCode,
		Success:    true,
		Chunks:     make([]string, 0),
		Events:     make([]SSEEvent, 0),
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	firstChunk := true

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				result.FinishedAt = time.Now()
				result.TotalTime = time.Since(start)
				break
			}

			if firstChunk {
				result.TTFT = time.Since(start)
				firstChunk = false
			}

			var chunk struct {
				Answer  string `json:"answer"`
				Content string `json:"content"`
				Error   string `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				content := chunk.Answer
				if content == "" {
					content = chunk.Content
				}
				if chunk.Error != "" {
					result.Error = chunk.Error
					result.Success = false
				}
				if content != "" {
					result.Chunks = append(result.Chunks, content)
					result.TotalChunks++
				}
			}

			result.Events = append(result.Events, SSEEvent{
				Type: "data",
				Data: data,
				Raw:  line,
			})
			result.Bytes += len(line)
		}
	}

	if result.FinishedAt.IsZero() {
		result.TotalTime = time.Since(start)
	}

	return result, scanner.Err()
}

func (c *SSEClient) BaseURL() string {
	return c.baseURL
}
