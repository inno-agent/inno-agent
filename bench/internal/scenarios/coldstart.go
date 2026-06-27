package scenarios

import (
	"context"
	"fmt"
	"time"

	"bench/internal/client"
	"bench/internal/config"
	"bench/internal/metrics"
	"bench/internal/report"
)

func RunColdStart(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "coldstart",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"message": cfg.Message,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)
	collector := metrics.NewCollector()

	start := time.Now()
	resp, err := httpClient.Chat(ctx, []client.Message{
		{Role: "user", Content: cfg.Message},
	})
	latency := time.Since(start)

	success := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	collector.Record(metrics.RequestRecord{
		Success:    success,
		StatusCode: resp.StatusCode,
		Latency:    latency,
		Bytes:      resp.Bytes,
		Error:      errStr,
	})

	result.FinishedAt = time.Now()
	s := collector.Snapshot()
	result.Summary = s
	result.Raw = map[string]any{
		"first_request": map[string]any{
			"latency_ms": float64(latency.Microseconds()) / 1000.0,
			"bytes":      resp.Bytes,
			"status":     resp.StatusCode,
		},
	}

	return result, nil
}

func RunWarmStart(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "warmstart",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"message": cfg.Message,
			"runs":    10,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)
	collector := metrics.NewCollector()

	runs := 10

	for i := 0; i < runs; i++ {
		start := time.Now()
		resp, err := httpClient.Chat(ctx, []client.Message{
			{Role: "user", Content: cfg.Message},
		})
		latency := time.Since(start)

		success := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}

		collector.Record(metrics.RequestRecord{
			Success:    success,
			StatusCode: resp.StatusCode,
			Latency:    latency,
			Bytes:      resp.Bytes,
			Error:      errStr,
		})
	}

	result.FinishedAt = time.Now()
	s := collector.Snapshot()
	result.Summary = s

	return result, nil
}

func generatePrompt(tokens int) string {
	word := "The quick brown fox jumps over the lazy dog. "
	wordLen := len(word)
	needed := tokens * 4
	repeats := needed/wordLen + 1
	result := ""
	for i := 0; i < repeats; i++ {
		result += word
		if len(result) >= needed {
			break
		}
	}
	if len(result) > needed {
		result = result[:needed]
	}
	return result
}

func generateMessages(tokenCount int) []client.Message {
	prompt := generatePrompt(tokenCount)
	return []client.Message{
		{Role: "user", Content: fmt.Sprintf("Please respond to this: %s", prompt)},
	}
}
