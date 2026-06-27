package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Target     string        `yaml:"target"      cli:"target"`
	Scenario   string        `yaml:"scenario"    cli:"scenario"`
	Users      int           `yaml:"users"       cli:"users"`
	RPS        int           `yaml:"rps"         cli:"rps"`
	Duration   time.Duration `yaml:"duration"    cli:"duration"`
	RampUp     time.Duration `yaml:"ramp_up"     cli:"ramp-up"`
	Timeout    time.Duration `yaml:"timeout"     cli:"timeout"`
	Stream     bool          `yaml:"stream"      cli:"stream"`
	Output     string        `yaml:"output"      cli:"output"`
	Message    string        `yaml:"message"`
	ChatID     string        `yaml:"chat_id"`
	ModelName  string        `yaml:"model_name"  cli:"model"`
	ConcLevels []int         `yaml:"conc_levels"`
	SpikeRPS   []int         `yaml:"spike_rps"`
	LongPrompt []int         `yaml:"long_prompt_tokens"`
}

func DefaultConfig() *Config {
	return &Config{
		Target:   "http://localhost:8080",
		Scenario: "smoke",
		Users:    1,
		RPS:      1,
		Duration: 30 * time.Second,
		RampUp:   5 * time.Second,
		Timeout:  180 * time.Second,
		Stream:   true,
		Output:   "results",
		Message:  "Hello, respond with a short greeting.",
		ChatID:   "new",
		ModelName: "",
		ConcLevels: []int{1, 2, 5, 10, 20, 30, 40, 50, 75, 100},
		SpikeRPS:   []int{1, 50, 100},
		LongPrompt: []int{500, 2000, 4000, 8000},
	}
}

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func (c *Config) MergeFlags(flags *Flags) {
	if flags.Target != "" {
		c.Target = flags.Target
	}
	if flags.Scenario != "" {
		c.Scenario = flags.Scenario
	}
	if flags.Users > 0 {
		c.Users = flags.Users
	}
	if flags.RPS > 0 {
		c.RPS = flags.RPS
	}
	if flags.Duration > 0 {
		c.Duration = flags.Duration
	}
	if flags.RampUp > 0 {
		c.RampUp = flags.RampUp
	}
	if flags.Timeout > 0 {
		c.Timeout = flags.Timeout
	}
	if flags.Output != "" {
		c.Output = flags.Output
	}
	if flags.Message != "" {
		c.Message = flags.Message
	}
	if flags.ConfigFile != "" {
		c.ChatID = flags.ChatID
	}
}

type Flags struct {
	Target     string
	Scenario   string
	Users      int
	RPS        int
	Duration   time.Duration
	RampUp     time.Duration
	Timeout    time.Duration
	Output     string
	Message    string
	ChatID     string
	ConfigFile string
	Stream     *bool
	Help       bool
}
