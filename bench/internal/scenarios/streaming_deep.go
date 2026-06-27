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

func RunStreamingDeep(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "streaming-deep",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"users":   cfg.Users,
			"message": cfg.Message,
		},
	}

	levels := cfg.ConcLevels
	if len(levels) == 0 {
		levels = []int{1, 2, 5, 10, 20}
	}

	var totalErrors atomic.Int64
	var totalRequests atomic.Int64
	totalCollector := metrics.NewCollector()

	for _, level := range levels {
		if ctx.Err() != nil {
			break
		}

		levelCollector := metrics.NewCollector()
		var wg sync.WaitGroup
		runs := 5

		for i := 0; i < level; i++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()
				for j := 0; j < runs; j++ {
					if ctx.Err() != nil {
						return
					}

					sseClient := client.NewSSEClient(cfg.Target, cfg.Timeout)
					start := time.Now()
					sseResult, err := sseClient.Stream(ctx, []client.Message{
						{Role: "user", Content: cfg.Message},
					})
					latency := time.Since(start)

					if err != nil {
						totalErrors.Add(1)
						totalRequests.Add(1)
						levelCollector.Record(metrics.RequestRecord{
							Success: false,
							Latency: latency,
							Error:   err.Error(),
						})
						continue
					}

					totalRequests.Add(1)

					genTime := time.Duration(0)
					if sseResult.TotalTime > sseResult.TTFT {
						genTime = sseResult.TotalTime - sseResult.TTFT
					}

					levelCollector.Record(metrics.RequestRecord{
						Success:         sseResult.Success,
						StatusCode:      sseResult.StatusCode,
						Latency:         latency,
						TTFT:            sseResult.TTFT,
						PromptTime:      sseResult.TTFT,
						GenerationTime:  genTime,
						StreamDuration:  sseResult.TotalTime,
						Bytes:           sseResult.Bytes,
						Chunks:          sseResult.TotalChunks,
						Error:           sseResult.Error,
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

		for _, r := range levelCollector.Records() {
			totalCollector.Record(r)
		}
	}

	result.FinishedAt = time.Now()
	result.Summary = totalCollector.Snapshot()
	result.Summary.TotalRequests = int(totalRequests.Load())
	result.Summary.FailedRequests = int(totalErrors.Load())
	if result.Summary.TotalRequests > 0 {
		result.Summary.SuccessRate = float64(result.Summary.TotalRequests-result.Summary.FailedRequests) / float64(result.Summary.TotalRequests) * 100
	}

	return result, nil
}
