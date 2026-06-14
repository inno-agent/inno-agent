package orchestrator

import (
	"context"
	"strings"
	"time"

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
	answer, err := o.provider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	ch := make(chan string, 4)
	go func() {
		defer close(ch)
		words := strings.Fields(answer)
		for i, w := range words {
			select {
			case <-ctx.Done():
				return
			case ch <- w + " ":
			}
			if i < len(words)-1 {
				time.Sleep(30 * time.Millisecond)
			}
		}
	}()
	return ch, nil
}
