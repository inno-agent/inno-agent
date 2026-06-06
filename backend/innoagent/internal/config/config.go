package config

import (
	"fmt"
	"os"
)

type Config struct {
	BaseURL    string
	Model      string
	ServerPort string
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

	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = os.Getenv("LLM_MODEL")
	}
	if model == "" {
		model = "qwen2.5:0.5b"
	}

	serverPort := os.Getenv("API_PORT")
	if serverPort == "" {
		serverPort = os.Getenv("SERVER_PORT")
	}
	if serverPort == "" {
		serverPort = "8080"
	}

	return Config{
		BaseURL:    baseURL,
		Model:      model,
		ServerPort: serverPort,
	}
}
