package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

type DockerStats struct {
	Timestamp  time.Time             `json:"timestamp"`
	Containers []ContainerStats      `json:"containers"`
}

type ContainerStats struct {
	Name        string  `json:"name"`
	CPU         string  `json:"cpu"`
	MemUsage    string  `json:"mem_usage"`
	MemPercent  string  `json:"mem_percent"`
	NetIO       string  `json:"net_io"`
	BlockIO     string  `json:"block_io"`
	PIDs        string  `json:"pids"`
}

func CollectDockerStats(ctx context.Context) (*DockerStats, error) {
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format", "{{json .}}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	stats := &DockerStats{
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

		stats.Containers = append(stats.Containers, ContainerStats{
			Name:       raw.Name,
			CPU:        raw.CPU,
			MemUsage:   raw.MemUsage,
			MemPercent: raw.MemPerc,
			NetIO:      raw.NetIO,
			BlockIO:    raw.BlockIO,
			PIDs:       raw.PIDs,
		})
	}

	return stats, nil
}
