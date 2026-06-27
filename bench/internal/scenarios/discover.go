package scenarios

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"bench/internal/client"
	"bench/internal/config"
	"bench/internal/metrics"
	"bench/internal/report"
)

type DiscoverResult struct {
	MaxStableConcurrency int     `json:"max_stable_concurrency"`
	MaxStableRPS         float64 `json:"max_stable_rps"`
	SaturationPoint      int     `json:"saturation_point"`
	RecommendedMaxUsers  int     `json:"recommended_max_users"`
	StopReason           string  `json:"stop_reason"`
	StopAtLevel          int     `json:"stop_at_level"`
}

func RunDiscover(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "discover",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"message": cfg.Message,
			"stream":  cfg.Stream,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)
	sseClient := client.NewSSEClient(cfg.Target, cfg.Timeout)

	levels := []int{1, 2, 5, 10, 20, 30, 40, 50, 75, 100}
	levelDuration := 30 * time.Second
	if cfg.Duration > 0 {
		levelDuration = cfg.Duration
	}

	maxP95 := 10000.0
	maxErrorRate := 1.0

	var totalErrors atomic.Int64
	var totalRequests atomic.Int64
	totalCollector := metrics.NewCollector()

	discover := DiscoverResult{
		MaxStableConcurrency: 0,
		MaxStableRPS:         0,
		SaturationPoint:      0,
		RecommendedMaxUsers:  0,
		StopReason:           "completed_all_levels",
	}

	latencyGrowth := make([]float64, 0, len(levels))
	var prevP95 float64

	for _, level := range levels {
		if ctx.Err() != nil {
			break
		}

		fmt.Printf("\n--- Testing %d concurrent users for %s ---\n", level, levelDuration)

		levelCollector := metrics.NewCollector()
		var wg sync.WaitGroup
		requestsPerUser := 3

		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(time.Duration(float64(time.Second) / float64(level)))
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					for u := 0; u < level; u++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							for r := 0; r < requestsPerUser; r++ {
								if ctx.Err() != nil {
									return
								}
								start := time.Now()
								var latency time.Duration
								var success bool
								var statusCode int
								var bytes int
								var ttft time.Duration
								var errStr string

								if cfg.Stream {
									sseResult, err := sseClient.Stream(ctx, []client.Message{
										{Role: "user", Content: cfg.Message},
									})
									latency = time.Since(start)
									if err != nil {
										errStr = err.Error()
									} else {
										success = sseResult.Success
										statusCode = sseResult.StatusCode
										bytes = sseResult.Bytes
										ttft = sseResult.TTFT
										if !sseResult.Success && sseResult.Error != "" {
											errStr = sseResult.Error
										}
									}
								} else {
									resp, err := httpClient.Chat(ctx, []client.Message{
										{Role: "user", Content: cfg.Message},
									})
									latency = time.Since(start)
									if err != nil {
										errStr = err.Error()
									} else {
										success = resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
										if resp != nil {
											statusCode = resp.StatusCode
											bytes = resp.Bytes
										}
									}
								}

								if !success {
									totalErrors.Add(1)
								}
								totalRequests.Add(1)

								levelCollector.Record(metrics.RequestRecord{
									Success:    success,
									StatusCode: statusCode,
									Latency:    latency,
									TTFT:       ttft,
									Bytes:      bytes,
									Error:      errStr,
								})
							}
						}()
					}
				}
			}
		}()

		time.Sleep(levelDuration)
		close(done)
		wg.Wait()

		s := levelCollector.Snapshot()
		result.Levels = append(result.Levels, report.LevelResult{
			Level:   level,
			Summary: s,
		})

		for _, r := range levelCollector.Records() {
			totalCollector.Record(r)
		}

		fmt.Printf("  Avg: %.1f ms | P50: %.1f | P95: %.1f | P99: %.1f | RPS: %.1f | Errors: %d | Success: %.1f%%\n",
			s.AvgLatency.Seconds()*1000, s.P50, s.P95, s.P99, s.RPS, s.FailedRequests, s.SuccessRate)

		if prevP95 > 0 {
			growth := (s.P95 - prevP95) / prevP95 * 100
			latencyGrowth = append(latencyGrowth, growth)
		}
		prevP95 = s.P95

		if s.P95 > maxP95 {
			discover.StopReason = fmt.Sprintf("P95 exceeded %.0fms (was %.1fms)", maxP95, s.P95)
			discover.StopAtLevel = level
			break
		}

		errorRate := 100 - s.SuccessRate
		if errorRate > maxErrorRate {
			discover.StopReason = fmt.Sprintf("Error rate %.1f%% exceeded %.1f%%", errorRate, maxErrorRate)
			discover.StopAtLevel = level
			break
		}

		if s.Timeouts > 0 && float64(s.Timeouts)/float64(s.TotalRequests)*100 > 5 {
			discover.StopReason = "Timeout rate exceeded 5%"
			discover.StopAtLevel = level
			break
		}

		discover.MaxStableConcurrency = level
		discover.MaxStableRPS = s.RPS
	}

	for i, growth := range latencyGrowth {
		if growth > 50 {
			discover.SaturationPoint = levels[i]
			break
		}
	}

	if discover.SaturationPoint > 0 {
		discover.RecommendedMaxUsers = discover.SaturationPoint
	} else {
		discover.RecommendedMaxUsers = discover.MaxStableConcurrency
	}

	result.FinishedAt = time.Now()
	result.Summary = totalCollector.Snapshot()
	result.Summary.TotalRequests = int(totalRequests.Load())
	result.Summary.FailedRequests = int(totalErrors.Load())
	if result.Summary.TotalRequests > 0 {
		result.Summary.SuccessRate = float64(result.Summary.TotalRequests-result.Summary.FailedRequests) / float64(result.Summary.TotalRequests) * 100
	}

	result.Raw = discover

	fmt.Println("\n=== Discovery Results ===")
	fmt.Printf("Max Stable Concurrency:  %d\n", discover.MaxStableConcurrency)
	fmt.Printf("Max Stable RPS:          %.1f\n", discover.MaxStableRPS)
	fmt.Printf("Saturation Point:        %d\n", discover.SaturationPoint)
	fmt.Printf("Recommended Max Users:   %d\n", discover.RecommendedMaxUsers)
	fmt.Printf("Stop Reason:             %s\n", discover.StopReason)

	return result, nil
}
