package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"repo-abc123/main.go":       "package main",
		"repo-abc123/sub/util.go":   "package sub",
	})
	req := httptest.NewRequest(http.MethodPost, "/populate", bytes.NewReader(tgz))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handlePopulate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("populate: code = %d body = %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(workspaceDir, "main.go"))
	if err != nil || string(got) != "package main" {
		t.Fatalf("main.go not at workspace root: err=%v content=%q", err, string(got))
	}
	if _, err := os.Stat(filepath.Join(workspaceDir, "sub", "util.go")); err != nil {
		t.Fatalf("sub/util.go missing: %v", err)
	}
}

func TestPopulate_RejectsZipSlip(t *testing.T) {
	sandboxToken = "s3cret"
	workspaceDir = t.TempDir()
	tgz := makeTarGz(t, map[string]string{"repo/../../escape.txt": "pwned"})
	req := httptest.NewRequest(http.MethodPost, "/populate", bytes.NewReader(tgz))
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
	req := httptest.NewRequest(http.MethodPost, "/exec", strings.NewReader(`{"command":"echo hi"}`))
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
	req := httptest.NewRequest(http.MethodPost, "/write", strings.NewReader(`{"path":"../escape.txt","content":"x"}`))
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	handleWrite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("traversal: code = %d, want 400", rec.Code)
	}
}
