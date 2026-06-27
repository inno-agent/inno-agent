package scenarios

import (
	"context"
	"sync"
	"time"

	"bench/internal/client"
	"bench/internal/config"
	"bench/internal/metrics"
	"bench/internal/report"
)

func RunRecovery(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "recovery",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"message": cfg.Message,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)

	spikeRPS := 50
	spikeDuration := 30 * time.Second

	spikeCollector := metrics.NewCollector()
	recoveryCollector := metrics.NewCollector()

	var wg sync.WaitGroup
	done := make(chan struct{})

	interval := time.Duration(float64(time.Second) / float64(spikeRPS))

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

					statusCode := 0
					bytes := 0
					if resp != nil {
						statusCode = resp.StatusCode
						bytes = resp.Bytes
					}

					spikeCollector.Record(metrics.RequestRecord{
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

	time.Sleep(5 * time.Second)

	for i := 0; i < 10; i++ {
		if ctx.Err() != nil {
			break
		}

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

		recoveryCollector.Record(metrics.RequestRecord{
			Success:    success,
			StatusCode: statusCode,
			Latency:    latency,
			Bytes:      bytes,
			Error:      errStr,
		})

		time.Sleep(time.Second)
	}

	result.FinishedAt = time.Now()

	spikeSummary := spikeCollector.Snapshot()
	recoverySummary := recoveryCollector.Snapshot()

	result.Levels = append(result.Levels, report.LevelResult{
		Level:   spikeRPS,
		Summary: spikeSummary,
	})

	result.Summary = recoverySummary
	result.Summary.TotalRequests = spikeSummary.TotalRequests + recoverySummary.TotalRequests
	result.Summary.FailedRequests = spikeSummary.FailedRequests + recoverySummary.FailedRequests
	if result.Summary.TotalRequests > 0 {
		result.Summary.SuccessRate = float64(result.Summary.TotalRequests-result.Summary.FailedRequests) / float64(result.Summary.TotalRequests) * 100
	}

	return result, nil
}
