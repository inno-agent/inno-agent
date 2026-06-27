package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bench/internal/metrics"
)

func WriteMarkdown(dir string, results []*ScenarioResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	path := filepath.Join(dir, "report.md")
	var sb strings.Builder

	sb.WriteString("# Load Testing Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("## %s\n\n", r.Name))
		sb.WriteString(fmt.Sprintf("- **Started:** %s\n", r.StartedAt.Format("15:04:05")))
		sb.WriteString(fmt.Sprintf("- **Duration:** %s\n", r.Summary.Duration.Round(time.Millisecond)))
		sb.WriteString(fmt.Sprintf("- **Total Requests:** %d\n", r.Summary.TotalRequests))
		sb.WriteString(fmt.Sprintf("- **Success Rate:** %.1f%%\n", r.Summary.SuccessRate))
		sb.WriteString("\n")

		writeSummaryTable(&sb, &r.Summary)

		if len(r.Levels) > 0 {
			sb.WriteString("### Levels\n\n")
			sb.WriteString("| Level | Avg (ms) | P50 (ms) | P95 (ms) | P99 (ms) | RPS | Errors | Success Rate |\n")
			sb.WriteString("|-------|----------|----------|----------|----------|-----|--------|-------------|\n")
			for _, l := range r.Levels {
				sb.WriteString(fmt.Sprintf("| %d | %.1f | %.1f | %.1f | %.1f | %.1f | %d | %.1f%% |\n",
					l.Level,
					l.Summary.AvgLatency.Seconds()*1000,
					l.Summary.P50,
					l.Summary.P95,
					l.Summary.P99,
					l.Summary.RPS,
					l.Summary.FailedRequests,
					l.Summary.SuccessRate,
				))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	return path, nil
}

func writeSummaryTable(sb *strings.Builder, s *metrics.Summary) {
	sb.WriteString("### Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Avg Latency | %.1f ms |\n", s.AvgLatency.Seconds()*1000))
	sb.WriteString(fmt.Sprintf("| P50 | %.1f ms |\n", s.P50))
	sb.WriteString(fmt.Sprintf("| P95 | %.1f ms |\n", s.P95))
	sb.WriteString(fmt.Sprintf("| P99 | %.1f ms |\n", s.P99))
	sb.WriteString(fmt.Sprintf("| Min Latency | %.1f ms |\n", s.MinLatency))
	sb.WriteString(fmt.Sprintf("| Max Latency | %.1f ms |\n", s.MaxLatency))
	sb.WriteString(fmt.Sprintf("| RPS | %.1f |\n", s.RPS))
	sb.WriteString(fmt.Sprintf("| P50 TTFT | %.1f ms |\n", s.P50TTFT))
	sb.WriteString(fmt.Sprintf("| P95 TTFT | %.1f ms |\n", s.P95TTFT))
	sb.WriteString(fmt.Sprintf("| P99 TTFT | %.1f ms |\n", s.P99TTFT))
	sb.WriteString(fmt.Sprintf("| Success Rate | %.1f%% |\n", s.SuccessRate))
	sb.WriteString(fmt.Sprintf("| Total Bytes | %d |\n", s.TotalBytes))
	sb.WriteString(fmt.Sprintf("| Total Chunks | %d |\n", s.TotalChunks))
	sb.WriteString(fmt.Sprintf("| Server Errors | %d |\n", s.ServerErrors))
	sb.WriteString(fmt.Sprintf("| Timeouts | %d |\n", s.Timeouts))
	sb.WriteString("\n")
}
