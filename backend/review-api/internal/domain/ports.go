package domain

import "context"

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMProvider sends a prompt and returns a response from the language model.
type LLMProvider interface {
	Chat(ctx context.Context, messages []LLMMessage) (string, error)
}

// DiffProvider fetches pull request diffs from an external source.
type DiffProvider interface {
	GetPRDiff(ctx context.Context, prID string) (string, error)
}

// ReviewService generates AI-powered pull request reviews.
type ReviewService interface {
	ReviewPR(ctx context.Context, prID string, diff string) (string, error)
}
