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

func RunQueue(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "queue",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"message": cfg.Message,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)

	levels := []int{1, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 60, 70, 80, 90, 100}

	var totalErrors atomic.Int64
	var totalRequests atomic.Int64

	for _, level := range levels {
		if ctx.Err() != nil {
			break
		}

		levelCollector := metrics.NewCollector()
		interval := time.Duration(float64(time.Second) / float64(level))
		testDuration := 30 * time.Second

		var wg sync.WaitGroup
		done := make(chan struct{})

		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					wg.Add(1)
					go func() {
						defer wg.Done()

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
					}()
				}
			}
		}()

		time.Sleep(testDuration)
		close(done)
		wg.Wait()

		s := levelCollector.Snapshot()
		result.Levels = append(result.Levels, report.LevelResult{
			Level:   level,
			Summary: s,
		})

		if s.FailedRequests > 0 && s.FailedRequests > s.SuccessfulRequests/2 {
			break
		}
	}

	result.FinishedAt = time.Now()
	result.Summary.TotalRequests = int(totalRequests.Load())
	result.Summary.FailedRequests = int(totalErrors.Load())
	if result.Summary.TotalRequests > 0 {
		result.Summary.SuccessRate = float64(result.Summary.TotalRequests-int(totalErrors.Load())) / float64(result.Summary.TotalRequests) * 100
	}
	result.Summary.Duration = result.FinishedAt.Sub(result.StartedAt)

	return result, nil
}
