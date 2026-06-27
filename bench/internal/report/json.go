package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bench/internal/metrics"
)

type ScenarioResult struct {
	Name       string             `json:"name"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at"`
	Config     map[string]any     `json:"config"`
	Summary    metrics.Summary    `json:"summary"`
	Levels     []LevelResult      `json:"levels,omitempty"`
	Raw        any                `json:"raw,omitempty"`
}

type LevelResult struct {
	Level   int              `json:"level"`
	Summary metrics.Summary  `json:"summary"`
}

func WriteJSON(dir string, result *ScenarioResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	name := fmt.Sprintf("%s-%s.json",
		result.StartedAt.Format("2006-01-02"),
		result.Name,
	)
	path := filepath.Join(dir, name)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	return path, nil
}
