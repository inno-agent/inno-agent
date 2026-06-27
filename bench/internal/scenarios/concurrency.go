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

func RunConcurrency(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "concurrency",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"users":   cfg.Users,
			"levels":  cfg.ConcLevels,
			"message": cfg.Message,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)
 levels := cfg.ConcLevels
	if len(levels) == 0 {
		levels = []int{1, 2, 5, 10, 20, 30, 40, 50, 75, 100}
	}

	var totalErrors atomic.Int64
	var totalRequests atomic.Int64

	for _, level := range levels {
		if ctx.Err() != nil {
			break
		}

		levelCollector := metrics.NewCollector()
		var wg sync.WaitGroup
		var active atomic.Int64

		requestsPerUser := 5
		if level > 20 {
			requestsPerUser = 3
		}
		if level > 50 {
			requestsPerUser = 2
		}

		for i := 0; i < level; i++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()
				for j := 0; j < requestsPerUser; j++ {
					if ctx.Err() != nil {
						return
					}
					active.Add(1)
					start := time.Now()
					resp, err := httpClient.Chat(ctx, []client.Message{
						{Role: "user", Content: cfg.Message},
					})
					latency := time.Since(start)
					active.Add(-1)

					success := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
					errStr := ""
					if err != nil {
						errStr = err.Error()
					}
					if !success {
						totalErrors.Add(1)
					}
					totalRequests.Add(1)

					statusCode := 0
					bytes := 0
					if resp != nil {
						statusCode = resp.StatusCode
						bytes = resp.Bytes
					}

					levelCollector.Record(metrics.RequestRecord{
						Success:    success,
						StatusCode: statusCode,
						Latency:    latency,
						Bytes:      bytes,
						Error:      errStr,
					})
				}
			}(i)
		}

		wg.Wait()

		s := levelCollector.Snapshot()
		result.Levels = append(result.Levels, report.LevelResult{
			Level:   level,
			Summary: s,
		})
	}

	result.FinishedAt = time.Now()
	totalCollector := metrics.NewCollector()
	for _, l := range result.Levels {
		for i := 0; i < l.Summary.TotalRequests; i++ {
			totalCollector.Record(metrics.RequestRecord{
				Success: true,
			})
		}
	}
	result.Summary = totalCollector.Snapshot()
	result.Summary.TotalRequests = int(totalRequests.Load())
	result.Summary.FailedRequests = int(totalErrors.Load())
	if result.Summary.TotalRequests > 0 {
		result.Summary.SuccessRate = float64(result.Summary.TotalRequests-int(totalErrors.Load())) / float64(result.Summary.TotalRequests) * 100
	}

	return result, nil
}
