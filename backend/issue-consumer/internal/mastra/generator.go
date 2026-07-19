package mastra

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

var _ domain.Generator = (*Generator)(nil)

// Generator adapts the Mastra codegen client to the domain.Generator
// interface. It mirrors backend/review-consumer/internal/review/MastraReviewer.
type Generator struct {
	client *Client
	logger *zap.Logger
}

func NewGenerator(client *Client, logger *zap.Logger) *Generator {
	return &Generator{
		client: client,
		logger: logger.With(zap.String("layer", "generator"), zap.String("backend", "mastra")),
	}
}

func (g *Generator) Generate(ctx context.Context, ref domain.IssueRef) (*domain.GenerationResult, error) {
	result, err := g.client.Generate(ctx, ref)
	if err != nil {
		g.logger.Error("mastra codegen failed",
			zap.String("issue", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)),
			zap.Error(err))
		return nil, fmt.Errorf("generator: mastra: %w", err)
	}
	return result, nil
}
