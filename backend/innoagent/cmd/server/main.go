package main

import (
	"context"
	"encoding/json"
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
	Message string `json:"message"`
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
		json.NewEncoder(w).Encode(HealthResponse{
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

		if req.Message == "" {
			http.Error(w, `{"error":"message field is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
		defer cancel()

		answer, err := orch.Ask(ctx, req.Message)
		if err != nil {
			log.Printf("orchestrator error: %v", err)
			http.Error(w, `{"error":"model inference failed"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(ChatResponse{Answer: answer})
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
