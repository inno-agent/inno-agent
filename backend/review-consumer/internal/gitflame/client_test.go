package gitflame_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/gitflame"
)

func TestGetPRDiff_NotConfigured(t *testing.T) {
	c := gitflame.NewClient("", "")
	_, err := c.GetPRDiff(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})
	if err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestGetPRDiff_AssemblesPatches(t *testing.T) {
	patch := base64.StdEncoding.EncodeToString([]byte("@@ -1 +1 @@\n-old\n+new\n"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/files"):
			_ = json.NewEncoder(w).Encode([]map[string]string{{"name": "main.go"}})
		case strings.Contains(r.URL.Path, "/diff/"):
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"file_path": "main.go", "patch": patch, "is_binary": false},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := gitflame.NewClient(srv.URL, "secret")
	diff, err := c.GetPRDiff(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(diff, "main.go") || !strings.Contains(diff, "-old") {
		t.Fatalf("unexpected diff: %q", diff)
	}
}

func TestGetRawFile_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# AGENTS.md content"))
	}))
	defer srv.Close()

	c := gitflame.NewClient(srv.URL, "tok")
	content, found, err := c.GetRawFile(context.Background(), domain.PRRef{Owner: "o", Repo: "r", HeadSHA: "abc"}, "AGENTS.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if content != "# AGENTS.md content" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestGetRawFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := gitflame.NewClient(srv.URL, "tok")
	_, found, err := c.GetRawFile(context.Background(), domain.PRRef{Owner: "o", Repo: "r", HeadSHA: "abc"}, "AGENTS.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected found=false for 404")
	}
}

func TestPostPRComment_Created(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]string
		_ = json.NewDecoder(r.Body).Decode(&payload)
		gotBody = payload["body"]
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := gitflame.NewClient(srv.URL, "tok")
	err := c.PostPRComment(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 5}, "Great PR!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody != "Great PR!" {
		t.Fatalf("expected body %q, got %q", "Great PR!", gotBody)
	}
}

func TestPostPRComment_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := gitflame.NewClient(srv.URL, "mytoken")
	_ = c.PostPRComment(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "review")
	if gotAuth != "token mytoken" {
		t.Fatalf("expected %q, got %q", "token mytoken", gotAuth)
	}
}

func TestGetRawFile_NestedPathPreservesSlashes(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	}))
	defer srv.Close()

	c := gitflame.NewClient(srv.URL, "tok")
	_, _, err := c.GetRawFile(context.Background(), domain.PRRef{Owner: "o", Repo: "r", HeadSHA: "abc"}, "docs/sub/file.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Slashes between segments must be preserved, not percent-encoded.
	if !strings.Contains(gotPath, "docs/sub/file.md") {
		t.Fatalf("expected path to contain docs/sub/file.md, got %q", gotPath)
	}
	if strings.Contains(gotPath, "%2F") {
		t.Fatalf("slashes must not be percent-encoded, got %q", gotPath)
	}
}
