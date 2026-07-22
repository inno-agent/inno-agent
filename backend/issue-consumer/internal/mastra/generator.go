package mastra

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

var _ domain.Generator = (*Generator)(nil)

// generateClient is the slice of mastra.Client this generator needs. A local
// interface keeps the test from spinning up an HTTP server.
type generateClient interface {
	Generate(ctx context.Context, ref domain.IssueRef, delegatedToken string) (*domain.GenerationResult, error)
}

// Generator adapts the Mastra codegen client to the domain.Generator
// interface. It mirrors backend/review-consumer/internal/review/MastraReviewer.
type Generator struct {
	client      generateClient
	tokenSource domain.TokenSource
	logger      *zap.Logger
}

func NewGenerator(client generateClient, tokenSource domain.TokenSource, logger *zap.Logger) *Generator {
	return &Generator{
		client:      client,
		tokenSource: tokenSource,
		logger:      logger.With(zap.String("layer", "generator"), zap.String("backend", "mastra")),
	}
}

func (g *Generator) Generate(ctx context.Context, ref domain.IssueRef) (*domain.GenerationResult, error) {
	// Exchange the delegated token first. This is also the onboarding gate:
	// Token returns ErrNotOnboarded for an unregistered assigner, which the
	// processor turns into a comment + skip — the same behaviour as the
	// single-shot path. It MUST run before the agent is called.
	token, err := g.tokenSource.Token(ctx, ref)
	if err != nil {
		// Propagate unchanged: ErrNotOnboarded, ErrTransient and ErrPermanent
		// each mean something specific to the processor.
		return nil, err
	}

	result, err := g.client.Generate(ctx, ref, token)
	if err != nil {
		g.logger.Error("mastra codegen failed",
			zap.String("issue", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)),
			zap.Error(err))
		return nil, fmt.Errorf("generator: mastra: %w", err)
	}
	return result, nil
}
