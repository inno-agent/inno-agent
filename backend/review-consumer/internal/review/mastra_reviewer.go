package review

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/mastra"
)

var _ domain.Reviewer = (*MastraReviewer)(nil)

type MastraReviewer struct {
	mastraClient *mastra.Client
	logger       *zap.Logger
}

func NewMastraReviewer(mastraClient *mastra.Client, logger *zap.Logger) *MastraReviewer {
	return &MastraReviewer{
		mastraClient: mastraClient,
		logger:       logger.With(zap.String("layer", "review"), zap.String("backend", "mastra")),
	}
}

func (r *MastraReviewer) Review(ctx context.Context, ref domain.PRRef) (string, error) {
	result, err := r.mastraClient.Review(ctx, ref)
	if err != nil {
		r.logger.Error("mastra review failed",
			zap.String("pr", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)),
			zap.Error(err))
		return "", fmt.Errorf("review: mastra: %w", err)
	}
	return result, nil
}
