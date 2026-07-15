package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
