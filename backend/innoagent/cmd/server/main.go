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

func main() {
	cfg := config.Load()

	log.Printf("starting InnoAgent orchestrator")
	log.Printf("ollama base url: %s", cfg.BaseURL)
	log.Printf("model: %s", cfg.Model)
	log.Printf("api port: %s", cfg.ServerPort)

	cat, err := catalog.Load()
	if err != nil {
		log.Fatalf("catalog: %v", err)
	}

	provider := llm.NewQwenProvider(
		cfg.BaseURL,
		llm.WithModel(cfg.Model),
	)

	orch := orchestrator.New(provider)
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

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if len(req.Messages) == 0 {
			http.Error(w, `{"error":"messages field is required"}`, http.StatusBadRequest)
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

		ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
		defer cancel()

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
				return
			}

			ch, err := orch.AskStream(ctx, req.Messages, req.ModelName)
			if err != nil {
				data, _ := json.Marshal(map[string]string{"error": err.Error()})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Printf("failed to write error response: %v", err)
				}
				flusher.Flush()
				return
			}

			for chunk := range ch {
				data, _ := json.Marshal(map[string]string{"answer": chunk})
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					log.Printf("failed to write chunk: %v", err)
				}
				flusher.Flush()
			}
			if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
				log.Printf("failed to write DONE signal: %v", err)
			}
			flusher.Flush()
			return
		}

		w.Header().Set("Content-Type", "application/json")
		answer, err := orch.Ask(ctx, req.Messages, req.ModelName)
		if err != nil {
			log.Printf("orchestrator error: %v", err)
			http.Error(w, `{"error":"model inference failed"}`, http.StatusInternalServerError)
			return
		}

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

	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}
