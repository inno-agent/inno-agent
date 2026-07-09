package review

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/mastra"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

func TestMastraReviewer_Review_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"review_markdown": "# Good PR"}`))
	}))
	defer srv.Close()

	client := mastra.NewClient(srv.URL)
	tokenSrc := tokensource.NewStatic("test-token")
	reviewer := NewMastraReviewer(client, tokenSrc, zap.NewNop())

	result, err := reviewer.Review(context.Background(), domain.PRRef{
		Owner: "org", Repo: "repo", Index: 1, HeadSHA: "sha",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "# Good PR" {
		t.Fatalf("expected '# Good PR', got %q", result)
	}
}

func TestMastraReviewer_Review_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"review_markdown": "ok"}`))
	}))
	defer srv.Close()

	client := mastra.NewClient(srv.URL)
	tokenSrc := &failingTokenSource{err: domain.ErrTransient}
	reviewer := NewMastraReviewer(client, tokenSrc, zap.NewNop())

	_, err := reviewer.Review(context.Background(), domain.PRRef{
		Owner: "org", Repo: "repo", Index: 1, HeadSHA: "sha",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMastraReviewer_Review_MastraError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal"}`))
	}))
	defer srv.Close()

	client := mastra.NewClient(srv.URL)
	tokenSrc := tokensource.NewStatic("token")
	reviewer := NewMastraReviewer(client, tokenSrc, zap.NewNop())

	_, err := reviewer.Review(context.Background(), domain.PRRef{
		Owner: "org", Repo: "repo", Index: 1, HeadSHA: "sha",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

type failingTokenSource struct {
	err error
}

func (f *failingTokenSource) Token(_ context.Context, _ domain.PRRef) (string, error) {
	return "", f.err
}
