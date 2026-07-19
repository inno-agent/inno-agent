package mastra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

func TestClient_Generate_SendsBearerAndTrimsURL(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotIssueNumber int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path

		var req codegenRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotIssueNumber = req.IssueNumber

		_ = json.NewEncoder(w).Encode(codegenResponse{
			Summary: "ok",
			Files: []codegenFile{
				{Path: "main.py", Content: "print('hi')"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "secret-xyz")
	out, err := c.Generate(context.Background(), domain.IssueRef{
		Owner: "o", Repo: "r", Index: 7, Title: "t", Body: "b",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(out.Files) != 1 || out.Files[0].Path != "main.py" {
		t.Fatalf("files = %+v", out.Files)
	}
	if out.Summary != "ok" {
		t.Fatalf("summary = %q", out.Summary)
	}
	if gotAuth != "Bearer secret-xyz" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer secret-xyz")
	}
	if gotPath != "/codegen" {
		t.Fatalf("path = %q, want /codegen", gotPath)
	}
	if gotIssueNumber != 7 {
		t.Fatalf("issueNumber = %d, want 7", gotIssueNumber)
	}
}

func TestClient_Generate_NoFilesIsPermanentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(codegenResponse{Summary: "empty", Files: nil})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1})
	if err == nil {
		t.Fatal("expected error for no files")
	}
	// 200 with empty files is a permanent error (the agent produced nothing usable).
	if !isPermanent(err) {
		t.Fatalf("expected permanent error, got: %v", err)
	}
}

func TestClient_Generate_4xxIsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !isPermanent(err) {
		t.Fatalf("expected permanent error for 4xx, got: %v", err)
	}
}

func TestClient_Generate_504IsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1})
	if err == nil {
		t.Fatal("expected error for 504")
	}
	if !isTransient(err) {
		t.Fatalf("expected transient error for 504, got: %v", err)
	}
}

func isPermanent(err error) bool {
	for e := err; e != nil; {
		if e == domain.ErrPermanent {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

func isTransient(err error) bool {
	for e := err; e != nil; {
		if e == domain.ErrTransient {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
