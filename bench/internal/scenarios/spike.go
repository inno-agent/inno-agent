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

func RunSpike(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "spike",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":   cfg.Target,
			"spike_rps": cfg.SpikeRPS,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)
	levels := cfg.SpikeRPS
	if len(levels) == 0 {
		levels = []int{1, 50, 100}
	}

	var totalErrors atomic.Int64
	var totalRequests atomic.Int64

	for _, targetRPS := range levels {
		if ctx.Err() != nil {
			break
		}

		levelCollector := metrics.NewCollector()
		spikeDuration := 30 * time.Second
		interval := time.Duration(float64(time.Second) / float64(targetRPS))

		var wg sync.WaitGroup
		done := make(chan struct{})
		var active atomic.Int64

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
					active.Add(1)
					go func() {
						defer wg.Done()
						defer active.Add(-1)

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

		time.Sleep(spikeDuration)
		close(done)
		wg.Wait()

		s := levelCollector.Snapshot()
		result.Levels = append(result.Levels, report.LevelResult{
			Level:   targetRPS,
			Summary: s,
		})
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
