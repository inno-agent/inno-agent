package mastra

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

type stubTokenSource struct {
	token string
	err   error
}

func (s *stubTokenSource) Token(context.Context, domain.IssueRef) (string, error) {
	return s.token, s.err
}

// countingGenerateClient records whether the agent was called and with what token.
type countingGenerateClient struct {
	calls int
	token string
	resp  *domain.GenerationResult
}

func (c *countingGenerateClient) Generate(_ context.Context, _ domain.IssueRef, delegatedToken string) (*domain.GenerationResult, error) {
	c.calls++
	c.token = delegatedToken
	return c.resp, nil
}

func TestGeneratorSkipsClientWhenNotOnboarded(t *testing.T) {
	client := &countingGenerateClient{}
	src := &stubTokenSource{err: domain.ErrNotOnboarded}
	g := NewGenerator(client, src, zap.NewNop())

	_, err := g.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1})

	if err != domain.ErrNotOnboarded {
		t.Fatalf("err = %v, want ErrNotOnboarded propagated unchanged", err)
	}
	// The gate must fire BEFORE the agent is called. Returning the right error
	// after already sending the request would still leak work for an
	// unregistered author.
	if client.calls != 0 {
		t.Errorf("client called %d times, want 0 (gate must precede the call)", client.calls)
	}
}

func TestGeneratorForwardsToken(t *testing.T) {
	client := &countingGenerateClient{resp: &domain.GenerationResult{
		Summary: "test",
		Files: []domain.GeneratedFile{
			{Path: "main.py", Content: "print('hi')"},
		},
	}}
	src := &stubTokenSource{token: "user-token"}
	g := NewGenerator(client, src, zap.NewNop())

	got, err := g.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got == nil || got.Summary != "test" {
		t.Errorf("got %+v", got)
	}
	if client.token != "user-token" {
		t.Errorf("client received token %q, want user-token", client.token)
	}
}

func TestGeneratorPropagatesTransientTokenError(t *testing.T) {
	client := &countingGenerateClient{}
	src := &stubTokenSource{err: domain.ErrTransient}
	g := NewGenerator(client, src, zap.NewNop())

	_, err := g.Generate(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 1})
	if err != domain.ErrTransient {
		t.Fatalf("err = %v, want ErrTransient", err)
	}
	if client.calls != 0 {
		t.Errorf("client called %d times, want 0", client.calls)
	}
}
