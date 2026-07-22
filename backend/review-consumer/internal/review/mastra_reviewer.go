package review

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

var _ domain.Reviewer = (*MastraReviewer)(nil)

// reviewClient is the slice of mastra.Client this reviewer needs. A local
// interface keeps the test from spinning up an HTTP server.
type reviewClient interface {
	Review(ctx context.Context, ref domain.PRRef, delegatedToken string) (string, error)
}

type MastraReviewer struct {
	mastraClient reviewClient
	tokenSource  domain.TokenSource
	logger       *zap.Logger
}

func NewMastraReviewer(mastraClient reviewClient, tokenSource domain.TokenSource, logger *zap.Logger) *MastraReviewer {
	return &MastraReviewer{
		mastraClient: mastraClient,
		tokenSource:  tokenSource,
		logger:       logger.With(zap.String("layer", "review"), zap.String("backend", "mastra")),
	}
}

func (r *MastraReviewer) Review(ctx context.Context, ref domain.PRRef) (string, error) {
	// Exchange the delegated token first. This is also the onboarding gate:
	// Token returns ErrNotOnboarded for an unregistered assigner, which the
	// processor turns into a comment + skip — the same behaviour as the
	// single-shot path. It MUST run before the agent is called.
	token, err := r.tokenSource.Token(ctx, ref)
	if err != nil {
		// Propagate unchanged: ErrNotOnboarded, ErrTransient and ErrPermanent
		// each mean something specific to the processor.
		return "", err
	}

	result, err := r.mastraClient.Review(ctx, ref, token)
	if err != nil {
		r.logger.Error("mastra review failed",
			zap.String("pr", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)),
			zap.Error(err))
		return "", fmt.Errorf("review: mastra: %w", err)
	}
	return result, nil
}
