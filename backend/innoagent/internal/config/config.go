package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	BaseURL     string
	Models      []string
	Model       string
	ServerPort  string
	IdentityURL string
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

	identityURL := os.Getenv("IDENTITY_URL")
	if identityURL == "" {
		identityURL = "http://identity:8081"
	}

	return Config{
		BaseURL:     baseURL,
		Models:      models,
		Model:       models[0],
		ServerPort:  serverPort,
		IdentityURL: identityURL,
	}
}
