package service

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

const reviewSystemPrompt = `You are a senior software engineer performing pull request review.

Review the provided diff.

Return markdown report with sections:

# Summary

# Potential Bugs

# Security Issues

# Performance Issues

# Suggested Improvements

Provide concise and actionable feedback.`

var _ domain.ReviewService = (*ReviewService)(nil)

// ReviewService generates AI-powered pull request reviews.
type ReviewService struct {
	diffProvider domain.DiffProvider
	llm          domain.LLMProvider
	logger       *zap.Logger
}

// NewReviewService creates a ReviewService with the given dependencies.
func NewReviewService(diffProvider domain.DiffProvider, llm domain.LLMProvider, logger *zap.Logger) *ReviewService {
	return &ReviewService{
		diffProvider: diffProvider,
		llm:          llm,
		logger:       logger.With(zap.String("layer", "service")),
	}
}

// ReviewPR returns an AI-generated markdown review for the given pull request.
// When diff is non-empty it is used directly; otherwise the diff is fetched via DiffProvider.
func (s *ReviewService) ReviewPR(ctx context.Context, prID string, diff string) (string, error) {
	prID = strings.TrimSpace(prID)
	if prID == "" {
		return "", fmt.Errorf("ReviewPR: %w", domain.ErrValidation)
	}

	diff = strings.TrimSpace(diff)
	if diff == "" {
		var err error
		diff, err = s.diffProvider.GetPRDiff(ctx, prID)
		if err != nil {
			s.logger.Error("failed to fetch PR diff", zap.String("pr_id", prID), zap.Error(err))
			return "", fmt.Errorf("ReviewPR: fetch diff: %w", err)
		}
	}

	if diff == "" {
		return "", fmt.Errorf("ReviewPR: %w", domain.ErrDiffUnavailable)
	}

	messages := []domain.LLMMessage{
		{
			Role:    "system",
			Content: reviewSystemPrompt + "\n\nPR diff:\n" + diff,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("Review pull request %s.", prID),
		},
	}

	review, err := s.llm.Chat(ctx, messages)
	if err != nil {
		s.logger.Error("failed to generate review", zap.String("pr_id", prID), zap.Error(err))
		return "", fmt.Errorf("ReviewPR: llm chat: %w", err)
	}

	return review, nil
}
