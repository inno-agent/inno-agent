package domain

import (
	"context"
	"errors"
)

var (
	ErrPermanent    = errors.New("permanent error")
	ErrTransient    = errors.New("transient error")
	ErrNotOnboarded = errors.New("assigner not onboarded")
)

type PRRef struct {
	Owner    string
	Repo     string
	Index    int64
	HeadSHA  string
	Assigner string
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMProvider interface {
	Chat(ctx context.Context, messages []LLMMessage, modelName string) (string, error)
}

type SourceProvider interface {
	GetPRDiff(ctx context.Context, ref PRRef) (string, error)
	GetRawFile(ctx context.Context, ref PRRef, path string) (content string, found bool, err error)
}

type CommentPoster interface {
	PostPRComment(ctx context.Context, ref PRRef, body string) error
}

type Reviewer interface {
	Review(ctx context.Context, ref PRRef) (string, error)
}

type TokenSource interface {
	Token(ctx context.Context, ref PRRef) (token string, err error)
}
