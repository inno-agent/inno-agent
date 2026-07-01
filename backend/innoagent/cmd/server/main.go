package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

func perfLog(cfg config.Config, start time.Time, label string, extra ...any) {
	if !cfg.PerfLog {
		return
	}
	elapsed := time.Since(start)
	args := []any{label, "elapsed_ms", float64(elapsed.Microseconds()) / 1000.0}
	args = append(args, extra...)
	log.Printf("PERF %s", fmt.Sprint(args...))
}

func main() {
	cfg := config.Load()

	log.Printf("starting InnoAgent orchestrator")
	log.Printf("ollama base url: %s", cfg.BaseURL)
	log.Printf("model: %s", cfg.Model)
	log.Printf("api port: %s", cfg.ServerPort)
	log.Printf("max concurrent llm: %d", cfg.MaxConcurrentLLM)

	cat, err := catalog.Load(cfg.Models)
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}

	provider := llm.NewQwenProvider(
		cfg.BaseURL,
		llm.WithModel(cfg.Model),
	)

	routerProvider := llm.NewQwenProvider(
		cfg.BaseURL,
		llm.WithModel(cfg.RouterModel),
		llm.WithTemperature(0),
	)

	routes := make([]orchestrator.RouteInfo, len(cfg.Models))
	for i, id := range cfg.Models {
		routes[i] = orchestrator.RouteInfo{
			Name:        id,
			Description: cat.Description(id),
		}
	}

	orch := orchestrator.New(provider, routerProvider, routes, cfg.Models, cfg.MaxConcurrentLLM)
	identityClient := auth.NewClient(cfg.IdentityURL)

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

	mux.HandleFunc("/v1/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		reqStart := time.Now()

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
		perfLog(cfg, reqStart, "auth")

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

			streamStart := time.Now()
			ch, err := orch.AskStream(ctx, req.Messages, req.ModelName)
			if err != nil {
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Printf("failed to write error response: %v", err)
				}
				flusher.Flush()
				return
			}
			perfLog(cfg, streamStart, "ttfb")

			chunkCount := 0
			for chunk := range ch {
				data, _ := json.Marshal(map[string]string{"answer": chunk})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Printf("failed to write chunk: %v", err)
				}
				flusher.Flush()
				chunkCount++
			}
			if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
				log.Printf("failed to write DONE signal: %v", err)
			}
			flusher.Flush()
			perfLog(cfg, reqStart, "stream_done", "chunks", chunkCount)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		answer, err := orch.Ask(ctx, req.Messages, req.ModelName)
		if err != nil {
			log.Printf("orchestrator error: %v", err)
			http.Error(w, `{"error":"model inference failed"}`, http.StatusInternalServerError)
			return
		}
		perfLog(cfg, reqStart, "chat_done")

		_ = json.NewEncoder(w).Encode(ChatResponse{Answer: answer})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 200 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Preload model into VRAM to eliminate cold start penalty
	go func() {
		time.Sleep(3 * time.Second)
		log.Printf("preloading model %s ...", cfg.Model)
		start := time.Now()
		_, err := provider.Chat(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, cfg.Model)
		if err != nil {
			log.Printf("model preload failed (non-fatal): %v", err)
		} else {
			log.Printf("model preloaded in %v", time.Since(start).Round(time.Millisecond))
		}
	}()

	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}
