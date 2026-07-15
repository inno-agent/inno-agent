package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var sandboxToken string

// workspaceDir is the root for exec cwd and file read/write. Overridable via
// SANDBOX_WORKDIR (defaults to /workspace, which the container image creates).
var workspaceDir = "/workspace"

// maxArchiveBytes caps the total decompressed size accepted by /populate,
// guarding against decompression bombs.
const maxArchiveBytes = 512 << 20 // 512 MiB

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
	sandboxToken = os.Getenv("SANDBOX_TOKEN")
	if sandboxToken == "" {
		log.Fatal("SANDBOX_TOKEN is required")
	}
	if wd := os.Getenv("SANDBOX_WORKDIR"); wd != "" {
		workspaceDir = wd
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/exec", handleExec)
	mux.HandleFunc("/write", handleWrite)
	mux.HandleFunc("/read", handleRead)
	mux.HandleFunc("/populate", handlePopulate)
	mux.HandleFunc("/health", handleHealth)

	port := os.Getenv("SANDBOX_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Sandbox server listening on :%s\n", port)
	http.ListenAndServe(":"+port, corsMiddleware(mux))
}

func authorized(r *http.Request) bool {
	const p = "Bearer "
	h := r.Header.Get("Authorization")
	return strings.HasPrefix(h, p) && subtle.ConstantTimeCompare([]byte(h[len(p):]), []byte(sandboxToken)) == 1
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !authorized(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
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
	cmd.Dir = workspaceDir

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

	if !authorized(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
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

	fullPath := filepath.Join(workspaceDir, cleanPath)

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

	if !authorized(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
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

	fullPath := filepath.Join(workspaceDir, cleanPath)

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

// handlePopulate resets the workspace and extracts a gzip tarball (POST body)
// into it. Gitea/gitflame archives nest everything under a single top-level
// directory, which is stripped so files land at the workspace root.
func handlePopulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !authorized(r) {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		jsonError(w, "invalid gzip", http.StatusBadRequest)
		return
	}
	defer func() { _ = gz.Close() }()

	// Reset the workspace so stale files from a previous review don't linger.
	// Clear the CONTENTS, not the dir itself: the server runs non-root and cannot
	// recreate /workspace (its parent is root-owned).
	if err := clearDir(workspaceDir); err != nil {
		jsonError(w, fmt.Sprintf("reset workspace: %v", err), http.StatusInternalServerError)
		return
	}

	root := filepath.Clean(workspaceDir)
	tr := tar.NewReader(gz)
	var written int64
	var count int

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			jsonError(w, fmt.Sprintf("read tar: %v", err), http.StatusBadRequest)
			return
		}

		// Strip the leading path component (archive root dir).
		rel := stripFirstComponent(hdr.Name)
		if rel == "" {
			continue
		}

		// Zip-slip guard: the resolved path must stay within the workspace.
		target := filepath.Join(root, rel)
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			jsonError(w, "path traversal in archive", http.StatusBadRequest)
			return
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				jsonError(w, fmt.Sprintf("mkdir: %v", err), http.StatusInternalServerError)
				return
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				jsonError(w, fmt.Sprintf("mkdir: %v", err), http.StatusInternalServerError)
				return
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				jsonError(w, fmt.Sprintf("create file: %v", err), http.StatusInternalServerError)
				return
			}
			n, err := io.Copy(f, io.LimitReader(tr, maxArchiveBytes-written+1))
			_ = f.Close()
			if err != nil {
				jsonError(w, fmt.Sprintf("write file: %v", err), http.StatusInternalServerError)
				return
			}
			written += n
			if written > maxArchiveBytes {
				jsonError(w, "archive too large", http.StatusRequestEntityTooLarge)
				return
			}
			count++
		default:
			// Skip symlinks, devices, etc. — never materialize them in the sandbox.
			continue
		}
	}

	json.NewEncoder(w).Encode(map[string]int{"files": count})
}

// clearDir removes everything inside dir without removing dir itself, so a
// non-root process that owns dir (but not its parent) can reset it.
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0755)
		}
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// stripFirstComponent removes the leading path segment (e.g. "repo-sha/") and
// returns the remainder, cleaned. Returns "" if nothing remains.
func stripFirstComponent(name string) string {
	name = strings.TrimPrefix(filepath.ToSlash(name), "/")
	i := strings.IndexByte(name, '/')
	if i < 0 {
		return ""
	}
	return filepath.Clean(name[i+1:])
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Error{Error: msg})
}
