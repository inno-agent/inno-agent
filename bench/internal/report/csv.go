package report

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"bench/internal/metrics"
)

func WriteRequestCSV(dir string, result *ScenarioResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	name := fmt.Sprintf("%s-%s-requests.csv",
		result.StartedAt.Format("2006-01-02"),
		result.Name,
	)
	path := fmt.Sprintf("%s/%s", dir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"timestamp",
		"success",
		"status_code",
		"latency_ms",
		"ttft_ms",
		"prompt_time_ms",
		"generation_time_ms",
		"stream_duration_ms",
		"bytes",
		"chunks",
		"prompt_tokens",
		"generated_tokens",
		"error",
	})

	for _, l := range result.Levels {
		for _, r := range l.Summary.Errors {
			_ = r
		}
	}

	return path, nil
}

func WriteLevelCSV(dir string, result *ScenarioResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	name := fmt.Sprintf("%s-%s-levels.csv",
		result.StartedAt.Format("2006-01-02"),
		result.Name,
	)
	path := fmt.Sprintf("%s/%s", dir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"level",
		"total_requests",
		"successful",
		"failed",
		"timeouts",
		"server_errors",
		"avg_latency_ms",
		"p50_ms",
		"p90_ms",
		"p95_ms",
		"p99_ms",
		"min_latency_ms",
		"max_latency_ms",
		"stddev_ms",
		"rps",
		"tokens_per_sec",
		"success_rate",
		"p50_ttft_ms",
		"p95_ttft_ms",
		"p90_ttft_ms",
		"total_bytes",
		"total_chunks",
		"prompt_tokens",
		"generated_tokens",
	})

	for _, l := range result.Levels {
		s := &l.Summary
		w.Write([]string{
			strconv.Itoa(l.Level),
			strconv.Itoa(s.TotalRequests),
			strconv.Itoa(s.SuccessfulRequests),
			strconv.Itoa(s.FailedRequests),
			strconv.Itoa(s.Timeouts),
			strconv.Itoa(s.ServerErrors),
			formatFloat(s.AvgLatency.Seconds() * 1000),
			formatFloat(s.P50),
			formatFloat(s.P90),
			formatFloat(s.P95),
			formatFloat(s.P99),
			formatFloat(s.MinLatency),
			formatFloat(s.MaxLatency),
			formatFloat(s.StdDev),
			formatFloat(s.RPS),
			formatFloat(s.TokensPerSec),
			formatFloat(s.SuccessRate),
			formatFloat(s.P50TTFT),
			formatFloat(s.P95TTFT),
			formatFloat(s.P90TTFT),
			strconv.Itoa(s.TotalBytes),
			strconv.Itoa(s.TotalChunks),
			strconv.Itoa(s.TotalPromptTokens),
			strconv.Itoa(s.TotalGeneratedTokens),
		})
	}

	return path, nil
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func WriteSummaryCSV(dir string, results []*ScenarioResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	name := fmt.Sprintf("%s-summary.csv", time.Now().Format("2006-01-02"))
	path := fmt.Sprintf("%s/%s", dir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{
		"scenario",
		"total_requests",
		"successful",
		"failed",
		"avg_latency_ms",
		"p50_ms",
		"p90_ms",
		"p95_ms",
		"p99_ms",
		"rps",
		"tokens_per_sec",
		"success_rate",
		"duration_s",
		"p50_ttft_ms",
		"p95_ttft_ms",
	})

	for _, r := range results {
		s := &r.Summary
		w.Write([]string{
			r.Name,
			strconv.Itoa(s.TotalRequests),
			strconv.Itoa(s.SuccessfulRequests),
			strconv.Itoa(s.FailedRequests),
			formatFloat(s.AvgLatency.Seconds() * 1000),
			formatFloat(s.P50),
			formatFloat(s.P90),
			formatFloat(s.P95),
			formatFloat(s.P99),
			formatFloat(s.RPS),
			formatFloat(s.TokensPerSec),
			formatFloat(s.SuccessRate),
			formatFloat(s.Duration.Seconds()),
			formatFloat(s.P50TTFT),
			formatFloat(s.P95TTFT),
		})
	}

	return path, nil
}

func init() {
	_ = metrics.Summary{}
}
