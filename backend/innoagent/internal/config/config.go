package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func getBoolEnv(key string) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1" || v == "yes"
}

type Config struct {
	BaseURL          string
	Models           []string
	Model            string
	RouterModel      string
	ServerPort       string
	IdentityURL      string
	PerfLog          bool
	MaxConcurrentLLM int
}

func Load() Config {
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "ollama"
	}

	ollamaPort := os.Getenv("OLLAMA_PORT")
	if ollamaPort == "" {
		ollamaPort = "11434"
	}

	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%s/v1", ollamaHost, ollamaPort)
	}

	// LLM_MODELS is the single source of truth for which models exist. The first
	// entry is the default (used when a request omits the model).
	models := strings.Fields(os.Getenv("LLM_MODELS"))
	if len(models) == 0 {
		models = []string{"qwen2.5:0.5b"}
	}

	serverPort := os.Getenv("API_PORT")
	if serverPort == "" {
		serverPort = os.Getenv("SERVER_PORT")
	}
	if serverPort == "" {
		serverPort = "8080"
	}

	routerModel := os.Getenv("ROUTER_MODEL")
	if routerModel == "" {
		routerModel = "fauxpaslife/arch-router:1.5b"
	}

	identityURL := os.Getenv("IDENTITY_URL")
	if identityURL == "" {
		identityURL = "http://identity:8081"
	}

	maxConcurrent := 16
	if v := os.Getenv("MAX_CONCURRENT_LLM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrent = n
		}
	}

	return Config{
		BaseURL:          baseURL,
		Models:           models,
		Model:            models[0],
		RouterModel:      routerModel,
		ServerPort:       serverPort,
		IdentityURL:      identityURL,
		PerfLog:          getBoolEnv("PERF_LOG"),
		MaxConcurrentLLM: maxConcurrent,
	}
}
