package monitor

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DockerSample struct {
	Timestamp  time.Time          `json:"timestamp"`
	Containers []ContainerSample  `json:"containers"`
}

type ContainerSample struct {
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemUsageMB float64 `json:"mem_usage_mb"`
	MemPercent float64 `json:"mem_percent"`
	NetIO      string  `json:"net_io"`
	BlockIO    string  `json:"block_io"`
	PIDs       int     `json:"pids"`
}

type DockerCollector struct {
	mu       sync.Mutex
	samples  []DockerSample
	interval time.Duration
	cancel   context.CancelFunc
}

func NewDockerCollector(interval time.Duration) *DockerCollector {
	return &DockerCollector{
		samples:  make([]DockerSample, 0, 256),
		interval: interval,
	}
}

func (dc *DockerCollector) Start(ctx context.Context) {
	ctx, dc.cancel = context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(dc.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sample, err := collectOnce(ctx)
				if err != nil {
					continue
				}
				dc.mu.Lock()
				dc.samples = append(dc.samples, *sample)
				dc.mu.Unlock()
			}
		}
	}()
}

func (dc *DockerCollector) Stop() {
	if dc.cancel != nil {
		dc.cancel()
	}
}

func (dc *DockerCollector) Samples() []DockerSample {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	out := make([]DockerSample, len(dc.samples))
	copy(out, dc.samples)
	return out
}

func (dc *DockerCollector) WriteCSV(path string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"timestamp", "container", "cpu_percent", "mem_usage_mb", "mem_percent", "net_io", "block_io", "pids"})

	for _, s := range dc.samples {
		for _, c := range s.Containers {
			w.Write([]string{
				s.Timestamp.Format(time.RFC3339),
				c.Name,
				fmt.Sprintf("%.2f", c.CPUPercent),
				fmt.Sprintf("%.2f", c.MemUsageMB),
				fmt.Sprintf("%.2f", c.MemPercent),
				c.NetIO,
				c.BlockIO,
				strconv.Itoa(c.PIDs),
			})
		}
	}
	return nil
}

func collectOnce(ctx context.Context) (*DockerSample, error) {
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	sample := &DockerSample{
		Timestamp: time.Now(),
	}

	lines := bytes.Split(stdout.Bytes(), []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var raw struct {
			Name     string `json:"Name"`
			CPU      string `json:"CPUPerc"`
			MemUsage string `json:"MemUsage"`
			MemPerc  string `json:"MemPerc"`
			NetIO    string `json:"NetIO"`
			BlockIO  string `json:"BlockIO"`
			PIDs     string `json:"PIDs"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		sample.Containers = append(sample.Containers, ContainerSample{
			Name:       raw.Name,
			CPUPercent: parsePercent(raw.CPU),
			MemUsageMB: parseMemUsage(raw.MemUsage),
			MemPercent: parsePercent(raw.MemPerc),
			NetIO:      raw.NetIO,
			BlockIO:    raw.BlockIO,
			PIDs:       parseInt(raw.PIDs),
		})
	}

	return sample, nil
}

func parsePercent(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseMemUsage(s string) float64 {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "/")
	if len(parts) == 0 {
		return 0
	}
	mem := strings.TrimSpace(parts[0])
	mem = strings.TrimSuffix(mem, "GiB")
	mem = strings.TrimSuffix(mem, "MiB")
	mem = strings.TrimSuffix(mem, "KiB")
	mem = strings.TrimSpace(mem)
	v, _ := strconv.ParseFloat(mem, 64)
	return v
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	v, _ := strconv.Atoi(s)
	return v
}

func dirOf(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "."
	}
	return path[:idx]
}

func CollectDockerStats(ctx context.Context) (*DockerSample, error) {
	return collectOnce(ctx)
}
