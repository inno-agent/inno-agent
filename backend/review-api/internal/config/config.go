package config

import (
	"os"
	"time"
)

type Config struct {
	ServerPort      string
	OrchestratorURL string
	AuthServiceURL  string
	GitFlameBaseURL string
	GitFlameToken   string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
}

func Load() *Config {
	return &Config{
		ServerPort:      getEnv("SERVER_PORT", "8001"),
		OrchestratorURL: getEnv("ORCHESTRATOR_URL", "http://orchestrator:8080"),
		AuthServiceURL:  getEnvAllowEmpty("AUTH_SERVICE_URL", "http://identity:8081"),
		GitFlameBaseURL: getEnv("GITFLAME_BASE_URL", ""),
		GitFlameToken:   getEnv("GITFLAME_TOKEN", ""),
		ReadTimeout:     getDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    getDuration("WRITE_TIMEOUT", 0),
		IdleTimeout:     getDuration("IDLE_TIMEOUT", 120*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvAllowEmpty uses the fallback only when the variable is unset, not when empty.
func getEnvAllowEmpty(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return fallback
}
