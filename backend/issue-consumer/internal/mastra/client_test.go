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
			Branch:  "innoagent-issue-7",
			ChangedFiles: []codegenChangedFile{
				{Path: "main.py", Status: "A"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "secret-xyz")
	out, err := c.Generate(context.Background(), domain.IssueRef{
		Owner: "o", Repo: "r", Index: 7, Title: "t", Body: "b",
	}, "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out.Branch != "innoagent-issue-7" {
		t.Fatalf("branch = %q", out.Branch)
	}
	if len(out.ChangedFiles) != 1 || out.ChangedFiles[0].Path != "main.py" {
		t.Fatalf("changedFiles = %+v", out.ChangedFiles)
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

func TestClient_Generate_NoBranchIsPermanentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(codegenResponse{Summary: "empty", Branch: ""})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "")
	if err == nil {
		t.Fatal("expected error for no branch")
	}
	// 200 with no branch is a permanent error (the agent never pushed anything).
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
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "")
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
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "")
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

func TestGenerateSendsDelegatedTokenSeparateFromAuth(t *testing.T) {
	var gotAuth, gotDelegated string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotDelegated = r.Header.Get("X-Delegated-Token")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(codegenResponse{
			Summary: "s",
			Branch:  "innoagent-issue-1",
			ChangedFiles: []codegenChangedFile{
				{Path: "a.py", Status: "A"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "user-token")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if gotAuth != "Bearer shared-secret" {
		t.Errorf("Authorization = %q, want the shared secret", gotAuth)
	}
	if gotDelegated != "user-token" {
		t.Errorf("X-Delegated-Token = %q, want user-token", gotDelegated)
	}
}

func TestGenerateOmitsDelegatedHeaderWhenTokenEmpty(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["X-Delegated-Token"]
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(codegenResponse{
			Summary: "s",
			Branch:  "innoagent-issue-1",
			ChangedFiles: []codegenChangedFile{
				{Path: "a.py", Status: "A"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if present {
		t.Error("X-Delegated-Token sent despite empty token")
	}
}

func TestGenerateClassifies401AsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "expired")
	if !isTransient(err) {
		t.Fatalf("err = %v, want ErrTransient (an expired delegated token is retryable)", err)
	}
}

func TestGenerateClassifies403AsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	_, err := c.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1}, "valid")
	if !isPermanent(err) {
		t.Fatalf("err = %v, want ErrPermanent (token valid, model/quota refused)", err)
	}
}
