package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"innoagent/internal/config"
	"innoagent/internal/llm"
	"innoagent/internal/orchestrator"
)

type ChatRequest struct {
	Messages []llm.Message `json:"messages"`
	Stream   bool          `json:"stream"`
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

	provider := llm.NewQwenProvider(
		cfg.BaseURL,
		llm.WithModel(cfg.Model),
	)

	orch := orchestrator.New(provider)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:  "ok",
			Model:   cfg.Model,
			BaseURL: cfg.BaseURL,
		})
	})

	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if len(req.Messages) == 0 {
			http.Error(w, `{"error":"messages field is required"}`, http.StatusBadRequest)
			return
		}

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
			defer cancel()

			ch, err := orch.AskStream(ctx, req.Messages)
			if err != nil {
				if _, err := fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error()); err != nil {
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
		ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
		defer cancel()

		answer, err := orch.Ask(ctx, req.Messages)
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
