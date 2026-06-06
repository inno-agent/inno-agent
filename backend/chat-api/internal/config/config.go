package config

import (
    "os"
    "time"
)

type Config struct {
    DatabaseURL  string
    ServerPort   string
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
    IdleTimeout  time.Duration
}

func Load() *Config {
    return &Config{
        DatabaseURL:  getEnv("DATABASE_URL", ""),
        ServerPort:   getEnv("SERVER_PORT", "8000"),
        ReadTimeout:  getDuration("READ_TIMEOUT", 10*time.Second),
        WriteTimeout: getDuration("WRITE_TIMEOUT", 0),
        IdleTimeout:  getDuration("IDLE_TIMEOUT", 120*time.Second),
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
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