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

func RunSmoke(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "smoke",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target": cfg.Target,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)
	collector := metrics.NewCollector()

	checks := []struct {
		name   string
		method string
		path   string
	}{
		{"orchestrator-health", "GET", "/health"},
		{"chat-health", "GET", "/health"},
	}

	for _, check := range checks {
		start := time.Now()
		var statusCode int
		var err error

		if check.path == "/health" {
			statusCode, err = httpClient.HealthCheck(ctx)
		}

		latency := time.Since(start)
		success := err == nil && statusCode >= 200 && statusCode < 300

		collector.Record(metrics.RequestRecord{
			Success:    success,
			StatusCode: statusCode,
			Latency:    latency,
			Error:      fmt.Sprintf("%s: %v", check.name, err),
		})
	}

	if cfg.Message != "" {
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

		statusCode := 0
		bytes := 0
		if resp != nil {
			statusCode = resp.StatusCode
			bytes = resp.Bytes
		}

		collector.Record(metrics.RequestRecord{
			Success:    success,
			StatusCode: statusCode,
			Latency:    latency,
			Bytes:      bytes,
			Error:      errStr,
		})
	}

	result.FinishedAt = time.Now()
	s := collector.Snapshot()
	result.Summary = s

	return result, nil
}
