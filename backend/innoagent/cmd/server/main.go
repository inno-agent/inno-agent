package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"innoagent/internal/auth"
	"innoagent/internal/catalog"
	"innoagent/internal/config"
	"innoagent/internal/llm"
	"innoagent/internal/orchestrator"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
	"go.uber.org/zap"
)

type ChatRequest struct {
	Messages  []llm.Message `json:"messages"`
	ModelName string        `json:"model_name,omitempty"`
	Stream    bool          `json:"stream"`
}

type ChatResponse struct {
	Answer string `json:"answer"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Model   string `json:"model"`
	BaseURL string `json:"base_url"`
}

// OpenAI-compatible types for /v1/chat/completions endpoint
type OpenAIChatRequest struct {
	Model    string           `json:"model"`
	Messages []OpenAIMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Choices []OpenAIChoice `json:"choices"`
	Model   string         `json:"model"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int          `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIChatChunkResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Choices []OpenAIChunkChoice `json:"choices"`
	Model   string              `json:"model"`
}

type OpenAIChunkChoice struct {
	Index        int              `json:"index"`
	Delta        OpenAIChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type OpenAIChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code,omitempty"`
}

func main() {
	cfg := config.Load()

	log := logger.New("orchestrator")
	defer func() { _ = log.Sync() }()

	log.Info(
		"starting InnoAgent orchestrator",
		zap.String("ollama_base_url", cfg.BaseURL),
		zap.String("vllm_base_url", cfg.VLLMBaseURL),
		zap.String("model", cfg.Model),
		zap.String("api_port", cfg.ServerPort),
	)

	cat, err := catalog.Load(cfg.Models)
	if err != nil {
		log.Fatal("failed to load catalog", zap.Error(err))
	}

	// Ollama provider for lightweight models (Fast, General)
	ollamaProvider := llm.NewQwenProvider(
		cfg.BaseURL,
		llm.WithModel(cfg.Model),
		llm.WithAPIKey(cfg.APIKey),
	)

	// vLLM provider for code model (Qwen2.5-Coder-32B)
	vllmProvider := llm.NewQwenProvider(
		cfg.VLLMBaseURL+"/v1",
		llm.WithModel("qwen2.5-coder-32b"),
		llm.WithAPIKey(cfg.APIKey),
	)

	routerProvider := llm.NewQwenProvider(
		cfg.BaseURL,
		llm.WithModel(cfg.RouterModel),
		llm.WithTemperature(0),
		llm.WithAPIKey(cfg.APIKey),
	)

	routes := make([]orchestrator.RouteInfo, len(cfg.Models))
	for i, id := range cfg.Models {
		routes[i] = orchestrator.RouteInfo{
			Name:        id,
			Description: cat.Description(id),
		}
	}

	// Build vLLM model map from config (VLLM_MODELS env var)
	vllmModels := make(map[string]bool, len(cfg.VLLMModels))
	for _, m := range cfg.VLLMModels {
		vllmModels[m] = true
	}

	orch := orchestrator.New(ollamaProvider, vllmProvider, routerProvider, routes, cfg.Models, vllmModels, log)
	identityClient := auth.NewClient(cfg.IdentityURL)

	telemetry.Init("orchestrator")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tracingCleanup, err := tracing.Setup(ctx, "orchestrator")
	if err != nil {
		log.Fatal("tracing init", zap.Error(err))
	}
	defer tracingCleanup()

	mux := http.NewServeMux()

	modelsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cat)
	})
	mux.Handle("/v1/models", auth.Middleware(identityClient)(modelsHandler))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:  "ok",
			Model:   cfg.Model,
			BaseURL: cfg.BaseURL,
		})
	})

	// ─── OpenAI-compatible /v1/chat/completions endpoint ────────────────────────
	// Returns responses in OpenAI format so Mastra and other OpenAI clients can
	// use the orchestrator directly (instead of bypassing to vLLM).
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		token := auth.Bearer(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if _, err := identityClient.Validate(r.Context(), token); err != nil {
			if errors.Is(err, auth.ErrUnauthorized) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			} else {
				http.Error(w, `{"error":"identity unavailable"}`, http.StatusBadGateway)
			}
			return
		}

		var req OpenAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if len(req.Messages) == 0 {
			http.Error(w, `{"error":"messages field is required"}`, http.StatusBadRequest)
			return
		}

		// Convert OpenAI messages to our internal format
		messages := make([]llm.Message, len(req.Messages))
		for i, m := range req.Messages {
			messages[i] = llm.Message{Role: m.Role, Content: m.Content}
		}

		modelName := req.Model
		ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
		defer cancel()

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
				return
			}

			ch, err := orch.AskStream(ctx, messages, modelName)
			if err != nil {
				data, _ := json.Marshal(OpenAIErrorResponse{Error: OpenAIError{Message: err.Error()}})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Error("failed to write error response", zap.Error(err))
				}
				flusher.Flush()
				return
			}

			for chunk := range ch {
				chunkResp := OpenAIChatChunkResponse{
					ID:    "chatcmpl-stream",
					Model: modelName,
					Choices: []OpenAIChunkChoice{{
						Index: 0,
						Delta: OpenAIChunkDelta{Content: chunk},
					}},
				}
				data, _ := json.Marshal(chunkResp)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Error("failed to write chunk", zap.Error(err))
				}
				flusher.Flush()
			}
			if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
				log.Error("failed to write DONE signal", zap.Error(err))
			}
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		answer, err := orch.Ask(ctx, messages, modelName)
		if err != nil {
			log.Error("orchestrator error", zap.Error(err))
			http.Error(w, `{"error":"model inference failed"}`, http.StatusInternalServerError)
			return
		}

		resp := OpenAIChatResponse{
			ID:      "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Object:  "chat.completion",
			Model:   modelName,
			Choices: []OpenAIChoice{{Index: 0, Message: OpenAIMessage{Role: "assistant", Content: answer}, FinishReason: "stop"}},
			Usage:   OpenAIUsage{PromptTokens: 0, CompletionTokens: 0, TotalTokens: 0},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/v1/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Authenticate before doing any work (parsing the body, etc.).
		token := auth.Bearer(r)
		if token == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if _, err := identityClient.Validate(r.Context(), token); err != nil {
			if errors.Is(err, auth.ErrUnauthorized) {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			} else {
				http.Error(w, `{"error":"identity unavailable"}`, http.StatusBadGateway)
			}
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if len(req.Messages) == 0 {
			http.Error(w, `{"error":"messages field is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
		defer cancel()

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
				return
			}

			ch, err := orch.AskStream(ctx, req.Messages, req.ModelName)
			if err != nil {
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Error("failed to write error response", zap.Error(err))
				}
				flusher.Flush()
				return
			}

			for chunk := range ch {
				data, _ := json.Marshal(map[string]string{"answer": chunk})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Error("failed to write chunk", zap.Error(err))
				}
				flusher.Flush()
			}
			if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
				log.Error("failed to write DONE signal", zap.Error(err))
			}
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		answer, err := orch.Ask(ctx, req.Messages, req.ModelName)
		if err != nil {
			log.Error("orchestrator error", zap.Error(err))
			http.Error(w, `{"error":"model inference failed"}`, http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(ChatResponse{Answer: answer})
	})

	mux.Handle("/metrics", telemetry.Handler())

	srv := &http.Server{
		Addr: ":" + cfg.ServerPort,
		Handler: tracing.HTTPMiddleware(
			"orchestrator",
			logger.CorrelationID(
				logger.InjectLogger(log)(
					logger.RequestLogger()(
						telemetry.StdMiddleware("orchestrator", mux),
					),
				),
			),
		),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 200 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server listening", zap.String("port", cfg.ServerPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("forced shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}
