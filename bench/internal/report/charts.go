package report

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func GenerateCharts(dir string, result *ScenarioResult) ([]string, error) {
	chartDir := filepath.Join(dir, "charts")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	var paths []string

	if len(result.Levels) > 0 {
		if p, err := latencyChart(chartDir, result); err == nil {
			paths = append(paths, p)
		}
		if p, err := ttftChart(chartDir, result); err == nil {
			paths = append(paths, p)
		}
		if p, err := rpsChart(chartDir, result); err == nil {
			paths = append(paths, p)
		}
		if p, err := errorRateChart(chartDir, result); err == nil {
			paths = append(paths, p)
		}
		if p, err := tokensPerSecChart(chartDir, result); err == nil {
			paths = append(paths, p)
		}
	}

	return paths, nil
}

func GenerateChartsFromDocker(dir string, samples interface{}, name string) ([]string, error) {
	chartDir := filepath.Join(dir, "charts")
	if err := os.MkdirAll(chartDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return nil, nil
}

func latencyChart(dir string, result *ScenarioResult) (string, error) {
	p := plot.New()
	p.Title.Text = "Latency vs Concurrency"
	p.X.Label.Text = "Concurrent Users"
	p.Y.Label.Text = "Latency (ms)"
	p.Legend.Top = true

	pts50 := make(plotter.XYs, len(result.Levels))
	pts90 := make(plotter.XYs, len(result.Levels))
	pts95 := make(plotter.XYs, len(result.Levels))
	pts99 := make(plotter.XYs, len(result.Levels))

	for i, l := range result.Levels {
		pts50[i].X = float64(l.Level)
		pts50[i].Y = l.Summary.P50
		pts90[i].X = float64(l.Level)
		pts90[i].Y = l.Summary.P90
		pts95[i].X = float64(l.Level)
		pts95[i].Y = l.Summary.P95
		pts99[i].X = float64(l.Level)
		pts99[i].Y = l.Summary.P99
	}

	err := plotutil.AddLinePoints(p,
		"P50", pts50,
		"P90", pts90,
		"P95", pts95,
		"P99", pts99,
	)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "latency.png")
	if err := p.Save(8*vg.Inch, 5*vg.Inch, path); err != nil {
		return "", err
	}
	return path, nil
}

func ttftChart(dir string, result *ScenarioResult) (string, error) {
	p := plot.New()
	p.Title.Text = "TTFT vs Concurrency"
	p.X.Label.Text = "Concurrent Users"
	p.Y.Label.Text = "TTFT (ms)"

	pts50 := make(plotter.XYs, 0, len(result.Levels))
	pts95 := make(plotter.XYs, 0, len(result.Levels))

	for _, l := range result.Levels {
		if l.Summary.P50TTFT > 0 {
			pts50 = append(pts50, plotter.XY{X: float64(l.Level), Y: l.Summary.P50TTFT})
		}
		if l.Summary.P95TTFT > 0 {
			pts95 = append(pts95, plotter.XY{X: float64(l.Level), Y: l.Summary.P95TTFT})
		}
	}

	if len(pts50) == 0 {
		return "", nil
	}

	plotutil.AddLinePoints(p, "P50 TTFT", pts50, "P95 TTFT", pts95)

	path := filepath.Join(dir, "ttft.png")
	if err := p.Save(8*vg.Inch, 5*vg.Inch, path); err != nil {
		return "", err
	}
	return path, nil
}

func rpsChart(dir string, result *ScenarioResult) (string, error) {
	p := plot.New()
	p.Title.Text = "RPS vs Concurrency"
	p.X.Label.Text = "Concurrent Users"
	p.Y.Label.Text = "Requests/sec"

	pts := make(plotter.XYs, len(result.Levels))
	for i, l := range result.Levels {
		pts[i].X = float64(l.Level)
		pts[i].Y = l.Summary.RPS
	}

	plotutil.AddLinePoints(p, "RPS", pts)

	path := filepath.Join(dir, "rps.png")
	if err := p.Save(8*vg.Inch, 5*vg.Inch, path); err != nil {
		return "", err
	}
	return path, nil
}

func errorRateChart(dir string, result *ScenarioResult) (string, error) {
	p := plot.New()
	p.Title.Text = "Error Rate vs Concurrency"
	p.X.Label.Text = "Concurrent Users"
	p.Y.Label.Text = "Error Rate (%)"

	pts := make(plotter.XYs, len(result.Levels))
	for i, l := range result.Levels {
		pts[i].X = float64(l.Level)
		pts[i].Y = 100 - l.Summary.SuccessRate
	}

	plotutil.AddLinePoints(p, "Error Rate", pts)

	path := filepath.Join(dir, "error_rate.png")
	if err := p.Save(8*vg.Inch, 5*vg.Inch, path); err != nil {
		return "", err
	}
	return path, nil
}

func tokensPerSecChart(dir string, result *ScenarioResult) (string, error) {
	p := plot.New()
	p.Title.Text = "Tokens/sec vs Concurrency"
	p.X.Label.Text = "Concurrent Users"
	p.Y.Label.Text = "Tokens/sec"

	pts := make(plotter.XYs, 0, len(result.Levels))
	for _, l := range result.Levels {
		if l.Summary.TokensPerSec > 0 {
			pts = append(pts, plotter.XY{X: float64(l.Level), Y: l.Summary.TokensPerSec})
		}
	}

	if len(pts) == 0 {
		return "", nil
	}

	plotutil.AddLinePoints(p, "Tokens/sec", pts)

	path := filepath.Join(dir, "tokens_per_sec.png")
	if err := p.Save(8*vg.Inch, 5*vg.Inch, path); err != nil {
		return "", err
	}
	return path, nil
}

var _ = color.RGBA{}
