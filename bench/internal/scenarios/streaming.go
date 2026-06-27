package scenarios

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"bench/internal/client"
	"bench/internal/config"
	"bench/internal/metrics"
	"bench/internal/report"
)

func RunStreaming(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "streaming",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"users":   cfg.Users,
			"message": cfg.Message,
		},
	}

	sseClient := client.NewSSEClient(cfg.Target, cfg.Timeout)
	collector := metrics.NewCollector()

	users := cfg.Users
	if users <= 0 {
		users = 5
	}

	runs := 10
	var totalChunks atomic.Int64
	var totalErrors atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < users; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			for j := 0; j < runs; j++ {
				if ctx.Err() != nil {
					return
				}

				start := time.Now()
				result, err := sseClient.Stream(ctx, []client.Message{
					{Role: "user", Content: cfg.Message},
				})
				latency := time.Since(start)

				if err != nil {
					totalErrors.Add(1)
					collector.Record(metrics.RequestRecord{
						Success: false,
						Latency: latency,
						Error:   err.Error(),
					})
					continue
				}

				totalChunks.Add(int64(result.TotalChunks))

				collector.Record(metrics.RequestRecord{
					Success:    result.Success,
					StatusCode: result.StatusCode,
					Latency:    latency,
					TTFT:       result.TTFT,
					Bytes:      result.Bytes,
					Chunks:     result.TotalChunks,
					Error:      result.Error,
				})
			}
		}(i)
	}

	wg.Wait()

	result.FinishedAt = time.Now()
	s := collector.Snapshot()
	result.Summary = s

	return result, nil
}
