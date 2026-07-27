package review

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

type stubTokenSource struct {
	token string
	err   error
}

func (s *stubTokenSource) Token(context.Context, domain.PRRef) (string, error) {
	return s.token, s.err
}

// countingReviewClient records whether the agent was called and with what token.
type countingReviewClient struct {
	calls int
	token string
	resp  string
}

func (c *countingReviewClient) Review(_ context.Context, _ domain.PRRef, delegatedToken string) (string, error) {
	c.calls++
	c.token = delegatedToken
	return c.resp, nil
}

func TestMastraReviewerSkipsClientWhenNotOnboarded(t *testing.T) {
	client := &countingReviewClient{}
	src := &stubTokenSource{err: domain.ErrNotOnboarded}
	r := NewMastraReviewer(client, src, zap.NewNop())

	_, err := r.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})

	if !errors.Is(err, domain.ErrNotOnboarded) {
		t.Fatalf("err = %v, want ErrNotOnboarded propagated unchanged", err)
	}
	// The gate must fire BEFORE the agent is called. Returning the right error
	// after already sending the request would still leak work for an
	// unregistered author.
	if client.calls != 0 {
		t.Errorf("client called %d times, want 0 (gate must precede the call)", client.calls)
	}
}

func TestMastraReviewerForwardsToken(t *testing.T) {
	client := &countingReviewClient{resp: "review body"}
	src := &stubTokenSource{token: "user-token"}
	r := NewMastraReviewer(client, src, zap.NewNop())

	got, err := r.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if got != "review body" {
		t.Errorf("got %q", got)
	}
	if client.token != "user-token" {
		t.Errorf("client received token %q, want user-token", client.token)
	}
}

func TestMastraReviewerPropagatesTransientTokenError(t *testing.T) {
	client := &countingReviewClient{}
	src := &stubTokenSource{err: domain.ErrTransient}
	r := NewMastraReviewer(client, src, zap.NewNop())

	_, err := r.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1})
	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("err = %v, want ErrTransient", err)
	}
	if client.calls != 0 {
		t.Errorf("client called %d times, want 0", client.calls)
	}
}
