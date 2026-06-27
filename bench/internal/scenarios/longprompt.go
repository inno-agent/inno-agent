package scenarios

import (
	"context"
	"time"

	"bench/internal/client"
	"bench/internal/config"
	"bench/internal/metrics"
	"bench/internal/report"
)

func RunLongPrompt(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "longprompt",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":          cfg.Target,
			"token_levels":    cfg.LongPrompt,
			"message":         cfg.Message,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)

	levels := cfg.LongPrompt
	if len(levels) == 0 {
		levels = []int{500, 2000, 4000, 8000}
	}

	for _, tokenCount := range levels {
		if ctx.Err() != nil {
			break
		}

		messages := generateMessages(tokenCount)
		start := time.Now()
		resp, err := httpClient.Chat(ctx, messages)
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

		levelCollector := metrics.NewCollector()
		levelCollector.Record(metrics.RequestRecord{
			Success:    success,
			StatusCode: statusCode,
			Latency:    latency,
			Bytes:      bytes,
			Error:      errStr,
		})

		s := levelCollector.Snapshot()
		result.Levels = append(result.Levels, report.LevelResult{
			Level:   tokenCount,
			Summary: s,
		})
	}

	result.FinishedAt = time.Now()
	totalReqs := 0
	totalErrs := 0
	for _, l := range result.Levels {
		totalReqs += l.Summary.TotalRequests
		totalErrs += l.Summary.FailedRequests
	}
	result.Summary.TotalRequests = totalReqs
	result.Summary.FailedRequests = totalErrs
	if totalReqs > 0 {
		result.Summary.SuccessRate = float64(totalReqs-totalErrs) / float64(totalReqs) * 100
	}
	result.Summary.Duration = result.FinishedAt.Sub(result.StartedAt)

	return result, nil
}
