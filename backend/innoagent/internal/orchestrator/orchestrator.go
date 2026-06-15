package orchestrator

import (
	"context"

	"innoagent/internal/llm"
)

type AIOrchestrator struct {
	provider llm.Provider
}

func New(provider llm.Provider) *AIOrchestrator {
	return &AIOrchestrator{provider: provider}
}

func (o *AIOrchestrator) Ask(ctx context.Context, messages []llm.Message) (string, error) {
	return o.provider.Chat(ctx, messages)
}

func (o *AIOrchestrator) AskStream(ctx context.Context, messages []llm.Message) (<-chan string, error) {
	return o.provider.Stream(ctx, messages)
}
