package scenarios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"bench/internal/client"
	"bench/internal/config"
	"bench/internal/metrics"
	"bench/internal/report"
)

type OllamaParam struct {
	NumParallel  []int    `yaml:"num_parallel"`
	MaxQueue     []int    `yaml:"max_queue"`
	KeepAlive    []string `yaml:"keep_alive"`
}

type SweepResult struct {
	Params      map[string]string `json:"params"`
	Summary     metrics.Summary   `json:"summary"`
	Duration    time.Duration     `json:"duration"`
}

func RunOllamaSweep(ctx context.Context, cfg *config.Config) (*report.ScenarioResult, error) {
	result := &report.ScenarioResult{
		Name:      "ollama-sweep",
		StartedAt: time.Now(),
		Config: map[string]any{
			"target":  cfg.Target,
			"message": cfg.Message,
		},
	}

	httpClient := client.NewHTTPClient(cfg.Target, cfg.Timeout)

	numParallelValues := []int{1, 2, 4, 8, 16}
	maxQueueValues := []int{32, 64, 128, 256, 512}
	keepAliveValues := []string{"5m", "10m", "30m"}

	testDuration := 30 * time.Second
	concurrentUsers := 10

	type combo struct {
		NumParallel int
		MaxQueue    int
		KeepAlive   string
	}

	combos := make([]combo, 0)
	for _, np := range numParallelValues {
		for _, mq := range maxQueueValues {
			for _, ka := range keepAliveValues {
				combos = append(combos, combo{np, mq, ka})
			}
		}
	}

	fmt.Printf("Testing %d parameter combinations\n", len(combos))
	fmt.Printf("Each test: %d users for %s\n", concurrentUsers, testDuration)

	var sweepResults []SweepResult

	for i, c := range combos {
		if ctx.Err() != nil {
			break
		}

		fmt.Printf("\n[%d/%d] NUM_PARALLEL=%d MAX_QUEUE=%d KEEP_ALIVE=%s\n",
			i+1, len(combos), c.NumParallel, c.MaxQueue, c.KeepAlive)

		envVars := map[string]string{
			"OLLAMA_NUM_PARALLEL": fmt.Sprintf("%d", c.NumParallel),
			"OLLAMA_MAX_QUEUE":    fmt.Sprintf("%d", c.MaxQueue),
			"OLLAMA_KEEP_ALIVE":   c.KeepAlive,
		}

		if err := applyOllamaParams(envVars); err != nil {
			fmt.Printf("  Failed to apply params: %v\n", err)
			continue
		}

		time.Sleep(10 * time.Second)

		if !waitForOllama(ctx, cfg.Target) {
			fmt.Printf("  Ollama not ready, skipping\n")
			continue
		}

		levelCollector := metrics.NewCollector()
		var wg sync.WaitGroup
		var active atomic.Int64
		done := make(chan struct{})

		interval := time.Duration(float64(time.Second) / float64(concurrentUsers))

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
			Level:   i,
			Summary: s,
		})

		sweepResults = append(sweepResults, SweepResult{
			Params: map[string]string{
				"num_parallel": fmt.Sprintf("%d", c.NumParallel),
				"max_queue":    fmt.Sprintf("%d", c.MaxQueue),
				"keep_alive":   c.KeepAlive,
			},
			Summary:  s,
			Duration: testDuration,
		})

		fmt.Printf("  RPS: %.1f | Avg: %.1f ms | P95: %.1f ms | Errors: %d | Success: %.1f%%\n",
			s.RPS, s.AvgLatency.Seconds()*1000, s.P95, s.FailedRequests, s.SuccessRate)
	}

	result.FinishedAt = time.Now()
	result.Summary = result.Levels[0].Summary
	if len(result.Levels) > 0 {
		bestIdx := 0
		for i, l := range result.Levels {
			if l.Summary.RPS > result.Levels[bestIdx].Summary.RPS && l.Summary.SuccessRate > 95 {
				bestIdx = i
			}
		}
		result.Summary = result.Levels[bestIdx].Summary
	}
	result.Summary.Duration = result.FinishedAt.Sub(result.StartedAt)

	result.Raw = sweepResults

	fmt.Println("\n=== Sweep Complete ===")
	fmt.Printf("Total combinations tested: %d\n", len(sweepResults))

	return result, nil
}

func applyOllamaParams(envVars map[string]string) error {
	for key, value := range envVars {
		cmd := exec.Command("docker", "exec", "inno-agent-ollama-1", "sh", "-c",
			fmt.Sprintf("export %s=%s && echo 'Setting %s=%s'", key, value, key, value))
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	return nil
}

func waitForOllama(ctx context.Context, target string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	for i := 0; i < 20; i++ {
		if ctx.Err() != nil {
			return false
		}
		resp, err := client.Get(target + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return true
			}
		}
		time.Sleep(time.Second)
	}
	return false
}

type sweepResultJSON struct {
	Params  map[string]string `json:"params"`
	Summary json.RawMessage  `json:"summary"`
}
