package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeTarGz builds a gzip tarball from name->content entries.
func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPopulate_RequiresToken(t *testing.T) {
	sandboxToken = "s3cret"
	req := httptest.NewRequest(http.MethodPost, "/populate", bytes.NewReader([]byte("x")))
	rec := httptest.NewRecorder()
	handlePopulate(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("populate no token: code = %d, want 401", rec.Code)
	}
}

func TestPopulate_ExtractsStrippingRoot(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()
	// Gitea archives nest everything under a top-level dir; it must be stripped.
	tgz := makeTarGz(t, map[string]string{
		"repo-abc123/main.go":     "package main",
		"repo-abc123/sub/util.go": "package sub",
	})
	req := httptest.NewRequest(http.MethodPost, "/populate?run_id=test", bytes.NewReader(tgz))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handlePopulate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("populate: code = %d body = %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(workspaceDir, "test", "main.go"))
	if err != nil || string(got) != "package main" {
		t.Fatalf("main.go not at workspace root: err=%v content=%q", err, string(got))
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "test", "sub", "util.go")); err != nil {
		t.Fatalf("sub/util.go missing: %v", err)
	}
}

func TestPopulate_ClearsStaleFilesKeepingDir(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()
	// A leftover file from a previous review must be gone after populate, and
	// the per-run dir itself must survive (non-root can't recreate it).
	runDir := filepath.Join(workspaceDir, "test")
	os.MkdirAll(runDir, 0755)
	stale := filepath.Join(runDir, "stale.txt")
	if err := os.WriteFile(stale, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	tgz := makeTarGz(t, map[string]string{"repo-x/new.go": "package x"})
	req := httptest.NewRequest(http.MethodPost, "/populate?run_id=test", bytes.NewReader(tgz))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handlePopulate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("populate: code = %d body = %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file not cleared: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "new.go")); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
}

func TestPopulate_RejectsZipSlip(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()
	tgz := makeTarGz(t, map[string]string{"repo/../../escape.txt": "pwned"})
	req := httptest.NewRequest(http.MethodPost, "/populate?run_id=test", bytes.NewReader(tgz))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handlePopulate(rec, req)
	// Escaping entry must not land outside workspace.
	parent := filepath.Dir(workspaceDir)
	if _, err := os.Stat(filepath.Join(parent, "escape.txt")); err == nil {
		t.Fatal("zip-slip: file written outside workspace")
	}
}

func TestExec_RequiresToken(t *testing.T) {
	sandboxToken = "s3cret"
	req := httptest.NewRequest(http.MethodPost, "/exec", strings.NewReader(`{"command":"echo hi"}`))
	rec := httptest.NewRecorder()
	handleExec(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: code = %d, want 401", rec.Code)
	}
}

func TestExec_RejectsWrongToken(t *testing.T) {
	sandboxToken = "s3cret"
	req := httptest.NewRequest(http.MethodPost, "/exec", strings.NewReader(`{"command":"echo hi"}`))
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	handleExec(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: code = %d, want 401", rec.Code)
	}
}

func TestExec_RunsWithToken(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()
	req := httptest.NewRequest(http.MethodPost, "/exec?run_id=test", strings.NewReader(`{"command":"echo hi"}`))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handleExec(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("with token: code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hi") {
		t.Fatalf("stdout missing echo output: %s", rec.Body.String())
	}
}

func TestWrite_RequiresToken(t *testing.T) {
	sandboxToken = "s3cret"
	req := httptest.NewRequest(http.MethodPost, "/write", strings.NewReader(`{"path":"a.txt","content":"x"}`))
	rec := httptest.NewRecorder()
	handleWrite(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("write no token: code = %d, want 401", rec.Code)
	}
}

func TestWrite_RejectsTraversal(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()
	req := httptest.NewRequest(http.MethodPost, "/write?run_id=test", strings.NewReader(`{"path":"../escape.txt","content":"x"}`))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handleWrite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("traversal: code = %d, want 400", rec.Code)
	}
}

func TestRunsAreIsolated(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()

	writeToRun(t, "run-a", "shared.txt", "content-a")
	writeToRun(t, "run-b", "shared.txt", "content-b")

	if got := readFromRun(t, "run-a", "shared.txt"); got != "content-a" {
		t.Errorf("run-a sees %q, want content-a", got)
	}
	if got := readFromRun(t, "run-b", "shared.txt"); got != "content-b" {
		t.Errorf("run-b sees %q, want content-b (runs must not share a workspace)", got)
	}
}

func TestPopulateOnlyClearsItsOwnRun(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()

	writeToRun(t, "run-a", "keep.txt", "still here")

	tgz := makeTarGz(t, map[string]string{"repo/new.go": "package main"})
	req := httptest.NewRequest(http.MethodPost, "/populate?run_id=run-b", bytes.NewReader(tgz))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handlePopulate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("populate run-b: %d %s", rec.Code, rec.Body.String())
	}

	if got := readFromRun(t, "run-a", "keep.txt"); got != "still here" {
		t.Errorf("run-a file gone after run-b populate: %q", got)
	}
}

func TestRunIDValidationRejectsTraversal(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()

	for _, bad := range []string{"../escape", "a/b", "a b", "", "a/../b"} {
		req := httptest.NewRequest(http.MethodGet, "/read?run_id="+url.QueryEscape(bad)+"&path=x", nil)
		req.Header.Set("Authorization", "Bearer s3cret")
		rec := httptest.NewRecorder()
		handleRead(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("run_id %q: code = %d, want 400", bad, rec.Code)
		}
	}
	parent := filepath.Dir(workspaceDir)
	if _, err := os.Stat(filepath.Join(parent, "escape")); err == nil {
		t.Fatal("run_id traversal wrote outside workspace")
	}
}

func TestReapRemovesStaleRunsOnly(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()

	writeToRun(t, "fresh", "f.txt", "x")
	writeToRun(t, "stale", "s.txt", "x")
	staleDir := filepath.Join(workspaceDir, "stale")
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(staleDir, old, old); err != nil {
		t.Fatal(err)
	}

	reapStaleRuns(time.Hour)

	if _, err := os.Stat(filepath.Join(workspaceDir, "stale")); !os.IsNotExist(err) {
		t.Error("stale run not reaped")
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "fresh")); err != nil {
		t.Error("fresh run wrongly reaped")
	}
}

func writeToRun(t *testing.T, runID, path, content string) {
	t.Helper()
	body, _ := json.Marshal(WriteRequest{Path: path, Content: content})
	req := httptest.NewRequest(http.MethodPost, "/write?run_id="+runID, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+sandboxToken)
	rec := httptest.NewRecorder()
	handleWrite(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("write to %s: %d %s", runID, rec.Code, rec.Body.String())
	}
}

func readFromRun(t *testing.T, runID, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/read?run_id="+runID+"&path="+url.QueryEscape(path), nil)
	req.Header.Set("Authorization", "Bearer "+sandboxToken)
	rec := httptest.NewRecorder()
	handleRead(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read from %s: %d %s", runID, rec.Code, rec.Body.String())
	}
	var resp ReadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Content
}
