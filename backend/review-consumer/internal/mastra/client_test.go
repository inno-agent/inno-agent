package mastra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

func TestClient_Review_SendsBearerAndTrimsURL(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]string{"review_markdown": "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "secret-xyz")
	out, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if out != "ok" {
		t.Fatalf("markdown = %q", out)
	}
	if gotAuth != "Bearer secret-xyz" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer secret-xyz")
	}
}
