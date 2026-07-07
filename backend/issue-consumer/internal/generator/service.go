package generator

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/llm"
)

var _ domain.Generator = (*Service)(nil)

type Service struct {
	source      domain.IssueSource
	pusher      domain.CodePusher
	llmProvider domain.LLMProvider
	tokenSource domain.TokenSource
	model       string
	logger      *zap.Logger
}

func NewService(
	source domain.IssueSource,
	pusher domain.CodePusher,
	llmProvider domain.LLMProvider,
	tokenSource domain.TokenSource,
	model string,
	logger *zap.Logger,
) *Service {
	return &Service{
		source:      source,
		pusher:      pusher,
		llmProvider: llmProvider,
		tokenSource: tokenSource,
		model:       model,
		logger:      logger.With(zap.String("layer", "generator")),
	}
}

func (s *Service) Generate(ctx context.Context, ref domain.IssueRef) (*domain.GenerationResult, error) {
	title, body, err := s.source.GetIssue(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("generator: get issue: %w", err)
	}
	if title == "" {
		title = ref.Title
	}
	if body == "" {
		body = ref.Body
	}

	agentsMD := s.fetchOptional(ctx, ref, "AGENTS.md")
	readmeMD := s.fetchOptional(ctx, ref, "README.md")

	userMsg := fmt.Sprintf(
		"Repository: %s/%s\nDefault branch: %s\nIssue type: %s\nIssue #%d\nTitle: %s\n\nDescription:\n%s\n\n=== AGENTS.md ===\n%s\n\n=== README.md ===\n%s",
		ref.Owner, ref.Repo, ref.DefaultBranch, ref.IssueType, ref.Index, title, body, agentsMD, readmeMD,
	)

	messages := []domain.LLMMessage{
		{Role: "system", Content: codegenSystemPrompt},
		{Role: "user", Content: userMsg},
	}

	tok, err := s.tokenSource.Token(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("generator: get token: %w", err)
	}
	ctx = llm.ContextWithToken(ctx, tok)

	raw, err := s.llmProvider.Chat(ctx, messages, s.model)
	if err != nil {
		s.logger.Error(
			"llm chat failed",
			zap.String("issue", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("generator: llm chat: %w", err)
	}

	result, err := parseLLMOutput(raw)
	for attempt := 0; err != nil && attempt < 2; attempt++ {
		s.logger.Warn(
			"llm output parse failed; requesting repair",
			zap.String("issue", fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)),
			zap.Int("attempt", attempt+1),
			zap.Error(err),
			zap.String("raw_prefix", truncate(raw, 300)),
		)

		repairMessages := append(
			messages,
			domain.LLMMessage{Role: "assistant", Content: raw},
			domain.LLMMessage{Role: "user", Content: codegenRepairPrompt},
		)
		raw, err = s.llmProvider.Chat(ctx, repairMessages, s.model)
		if err != nil {
			return nil, fmt.Errorf("generator: llm repair chat: %w", err)
		}
		result, err = parseLLMOutput(raw)
	}

	if err != nil {
		return nil, fmt.Errorf("generator: parse llm output: %w: %w", err, domain.ErrPermanent)
	}
	if len(result.Files) == 0 {
		return nil, fmt.Errorf("generator: llm returned no files: %w", domain.ErrPermanent)
	}

	return result, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (s *Service) fetchOptional(ctx context.Context, ref domain.IssueRef, path string) string {
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
