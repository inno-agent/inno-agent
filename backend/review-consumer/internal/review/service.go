package review

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/llm"
)

const reviewSystemPrompt = `You are a senior software engineer performing pull request review.

Review the provided diff.

Return markdown report with sections:

( 1. ) Summary

( 2. ) Potential Bugs

( 3. ) Security Issues

( 4. ) Performance Issues

( 5. ) Suggested Improvements

Provide concise and actionable feedback.`

var _ domain.Reviewer = (*Service)(nil)

type Service struct {
	source      domain.SourceProvider
	llmProvider domain.LLMProvider
	tokenSource domain.TokenSource
	model       string
	logger      *zap.Logger
}

func NewService(source domain.SourceProvider, llmProvider domain.LLMProvider, tokenSource domain.TokenSource, model string, logger *zap.Logger) *Service {
	return &Service{
		source:      source,
		llmProvider: llmProvider,
		tokenSource: tokenSource,
		model:       model,
		logger:      logger.With(zap.String("layer", "review")),
	}
}

func (s *Service) Review(ctx context.Context, ref domain.PRRef) (string, error) {
	diff, err := s.source.GetPRDiff(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("review: get diff: %w", err)
	}

	agentsMD := s.fetchOptional(ctx, ref, "AGENTS.md")
	readmeMD := s.fetchOptional(ctx, ref, "README.md")
	description := s.fetchDescription(ctx, ref)

	userMsg := fmt.Sprintf(
		"PR Description:\n%s\n\nRepo context files (if present):\n\n=== AGENTS.md ===\n%s\n\n=== README.md ===\n%s\n\nReview pull request %s/%s#%d.\n\nDiff:\n%s",
		description, agentsMD, readmeMD, ref.Owner, ref.Repo, ref.Index, diff,
	)

	messages := []domain.LLMMessage{
		{Role: "system", Content: reviewSystemPrompt},
		{Role: "user", Content: userMsg},
	}

	tok, err := s.tokenSource.Token(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("review: get token: %w", err)
	}
	ctx = llm.ContextWithToken(ctx, tok)

	result, err := s.llmProvider.Chat(ctx, messages, s.model)
	if err != nil {
		s.logger.Error("llm chat failed", zap.String("pr", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)), zap.Error(err))
		return "", fmt.Errorf("review: llm chat: %w", err)
	}
	return result, nil
}

func (s *Service) fetchOptional(ctx context.Context, ref domain.PRRef, path string) string {
	content, found, err := s.source.GetRawFile(ctx, ref, path)
	if err != nil {
		s.logger.Warn("failed to fetch context file", zap.String("path", path), zap.Error(err))
		return "(absent)"
	}
	if !found {
		return "(absent)"
	}
	return content
}

func (s *Service) fetchDescription(ctx context.Context, ref domain.PRRef) string {
	desc, err := s.source.GetPRDescription(ctx, ref)
	if err != nil {
		s.logger.Warn("failed to fetch PR description", zap.Error(err))
		return "(no description)"
	}
	if desc == "" {
		return "(no description)"
	}
	return desc
}
