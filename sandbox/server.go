package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ExecRequest struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // seconds, default 60
}

type ExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"`
}

type WriteRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type ReadRequest struct {
	Path string `json:"path"`
}

type ReadResponse struct {
	Content string `json:"content"`
	Exists  bool   `json:"exists"`
}

type Error struct {
	Error string `json:"error"`
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/exec", handleExec)
	mux.HandleFunc("/write", handleWrite)
	mux.HandleFunc("/read", handleRead)
	mux.HandleFunc("/health", handleHealth)

	port := os.Getenv("SANDBOX_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Sandbox server listening on :%s\n", port)
	http.ListenAndServe(":"+port, corsMiddleware(mux))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		jsonError(w, "command is required", http.StatusBadRequest)
		return
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 60
	}
	if timeout > 300 {
		timeout = 300
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeout)*time.Second)
	defer cancel()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "bash", "-c", req.Command)
	cmd.Dir = "/workspace"

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	json.NewEncoder(w).Encode(ExecResponse{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	})
}

func handleWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		jsonError(w, "path is required", http.StatusBadRequest)
		return
	}

	// Prevent path traversal
	cleanPath := filepath.Clean(req.Path)
	if strings.Contains(cleanPath, "..") {
		jsonError(w, "path traversal not allowed", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join("/workspace", cleanPath)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		jsonError(w, fmt.Sprintf("failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		jsonError(w, fmt.Sprintf("failed to write file: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		jsonError(w, "path is required", http.StatusBadRequest)
		return
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		jsonError(w, "path traversal not allowed", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join("/workspace", cleanPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			json.NewEncoder(w).Encode(ReadResponse{Content: "", Exists: false})
			return
		}
		jsonError(w, fmt.Sprintf("failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(ReadResponse{Content: string(data), Exists: true})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Error{Error: msg})
}

// Unused but available for future use
var _ = io.Copy
