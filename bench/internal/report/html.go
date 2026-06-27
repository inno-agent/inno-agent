package report

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func WriteHTML(dir string, results []*ScenarioResult, chartPaths []string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	path := filepath.Join(dir, "report.html")
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Load Testing Report</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: #f5f5f5; color: #333; }
h1 { color: #1a1a2e; border-bottom: 3px solid #e94560; padding-bottom: 10px; }
h2 { color: #1a1a2e; margin-top: 40px; }
h3 { color: #0f3460; }
.summary { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin: 20px 0; }
table { border-collapse: collapse; width: 100%; margin: 10px 0; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
th { background: #1a1a2e; color: white; padding: 12px; text-align: left; }
td { padding: 10px 12px; border-bottom: 1px solid #eee; }
tr:hover { background: #f8f9fa; }
.chart { margin: 20px 0; text-align: center; }
.chart img { max-width: 100%; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
.metric { display: inline-block; margin: 10px 20px 10px 0; }
.metric-label { font-size: 12px; color: #666; text-transform: uppercase; }
.metric-value { font-size: 24px; font-weight: bold; color: #1a1a2e; }
.success { color: #2ecc71; }
.warning { color: #f39c12; }
.error { color: #e74c3c; }
.meta { color: #666; font-size: 14px; }
</style>
</head>
<body>
<h1>Load Testing Report</h1>
<p class="meta">Generated: `)

	sb.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	sb.WriteString("</p>\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("<h2>%s</h2>\n", r.Name))
		sb.WriteString(fmt.Sprintf("<p class='meta'>Started: %s | Duration: %s</p>\n",
			r.StartedAt.Format("15:04:05"),
			r.Summary.Duration.Round(time.Millisecond)))

		sb.WriteString("<div class='summary'>\n")
		sb.WriteString("<div class='metric'><div class='metric-label'>Total Requests</div>")
		sb.WriteString(fmt.Sprintf("<div class='metric-value'>%d</div></div>\n", r.Summary.TotalRequests))

		rateClass := "success"
		if r.Summary.SuccessRate < 99 {
			rateClass = "warning"
		}
		if r.Summary.SuccessRate < 90 {
			rateClass = "error"
		}
		sb.WriteString(fmt.Sprintf("<div class='metric'><div class='metric-label'>Success Rate</div><div class='metric-value %s'>%.1f%%</div></div>\n",
			rateClass, r.Summary.SuccessRate))

		sb.WriteString(fmt.Sprintf("<div class='metric'><div class='metric-label'>RPS</div><div class='metric-value'>%.1f</div></div>\n", r.Summary.RPS))
		sb.WriteString(fmt.Sprintf("<div class='metric'><div class='metric-label'>Tokens/sec</div><div class='metric-value'>%.1f</div></div>\n", r.Summary.TokensPerSec))
		sb.WriteString("</div>\n")

		sb.WriteString("<table>\n<tr><th>Metric</th><th>Value</th></tr>\n")
		writeHTMLRow(&sb, "Avg Latency", fmt.Sprintf("%.1f ms", r.Summary.AvgLatency.Seconds()*1000))
		writeHTMLRow(&sb, "P50", fmt.Sprintf("%.1f ms", r.Summary.P50))
		writeHTMLRow(&sb, "P90", fmt.Sprintf("%.1f ms", r.Summary.P90))
		writeHTMLRow(&sb, "P95", fmt.Sprintf("%.1f ms", r.Summary.P95))
		writeHTMLRow(&sb, "P99", fmt.Sprintf("%.1f ms", r.Summary.P99))
		writeHTMLRow(&sb, "StdDev", fmt.Sprintf("%.1f ms", r.Summary.StdDev))
		writeHTMLRow(&sb, "P50 TTFT", fmt.Sprintf("%.1f ms", r.Summary.P50TTFT))
		writeHTMLRow(&sb, "P95 TTFT", fmt.Sprintf("%.1f ms", r.Summary.P95TTFT))
		writeHTMLRow(&sb, "Failed Requests", fmt.Sprintf("%d", r.Summary.FailedRequests))
		writeHTMLRow(&sb, "Timeouts", fmt.Sprintf("%d", r.Summary.Timeouts))
		writeHTMLRow(&sb, "Server Errors", fmt.Sprintf("%d", r.Summary.ServerErrors))
		writeHTMLRow(&sb, "Total Bytes", fmt.Sprintf("%d", r.Summary.TotalBytes))
		writeHTMLRow(&sb, "Generated Tokens", fmt.Sprintf("%d", r.Summary.TotalGeneratedTokens))
		sb.WriteString("</table>\n")

		if len(r.Levels) > 0 {
			sb.WriteString("<h3>Level Breakdown</h3>\n")
			sb.WriteString("<table>\n<tr><th>Level</th><th>Avg (ms)</th><th>P50</th><th>P90</th><th>P95</th><th>P99</th><th>RPS</th><th>Tokens/s</th><th>Errors</th><th>Rate</th></tr>\n")
			for _, l := range r.Levels {
				sb.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%.1f</td><td>%d</td><td>%.1f%%</td></tr>\n",
					l.Level,
					l.Summary.AvgLatency.Seconds()*1000,
					l.Summary.P50,
					l.Summary.P90,
					l.Summary.P95,
					l.Summary.P99,
					l.Summary.RPS,
					l.Summary.TokensPerSec,
					l.Summary.FailedRequests,
					l.Summary.SuccessRate,
				))
			}
			sb.WriteString("</table>\n")
		}
	}

	if len(chartPaths) > 0 {
		sb.WriteString("<h2>Charts</h2>\n")
		for _, cp := range chartPaths {
			data, err := os.ReadFile(cp)
			if err != nil {
				continue
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			name := filepath.Base(cp)
			sb.WriteString(fmt.Sprintf("<div class='chart'><h3>%s</h3><img src='data:image/png;base64,%s' alt='%s'></div>\n",
				strings.TrimSuffix(name, ".png"), b64, name))
		}
	}

	sb.WriteString("</body>\n</html>")

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	return path, nil
}

func writeHTMLRow(sb *strings.Builder, label, value string) {
	sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>\n", label, value))
}
