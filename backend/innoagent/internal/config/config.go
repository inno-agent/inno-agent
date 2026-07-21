package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL     string
	APIKey      string
	Models      []string
	Model       string
	RouterModel string
	ServerPort  string
	IdentityURL string

	// CompletionsTimeout bounds a single /v1/chat/completions call to the
	// runtime. Agentic calls run longer than chat ones, so this is expected to
	// need raising.
	CompletionsTimeout time.Duration
	// MaxBodyBytes caps request and response bodies. Opaque passthrough buffers
	// both whole in memory, so an unbounded cap is an unbounded allocation.
	MaxBodyBytes int64
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

	apiKey := os.Getenv("OLLAMA_API_KEY")

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

	completionsTimeout := 180 * time.Second
	if v := os.Getenv("LLM_COMPLETIONS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			completionsTimeout = d
		}
	}

	var maxBodyBytes int64 = 10 << 20
	if v := os.Getenv("LLM_MAX_BODY_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxBodyBytes = n
		}
	}

	return Config{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Models:      models,
		Model:       models[0],
		RouterModel: routerModel,
		ServerPort:  serverPort,
		IdentityURL: identityURL,

		CompletionsTimeout: completionsTimeout,
		MaxBodyBytes:       maxBodyBytes,
	}
}
