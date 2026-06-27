package domain

import (
	"context"
	"errors"
)

// ErrPermanent wraps errors that should NOT be retried (e.g. 4xx responses).
// Callers check with errors.Is(err, domain.ErrPermanent).
var ErrPermanent = errors.New("permanent error")

// ErrTransient wraps errors that are safe to retry (e.g. 5xx, network faults).
var ErrTransient = errors.New("transient error")

// ErrNotOnboarded is returned when the assigner has not linked their account.
var ErrNotOnboarded = errors.New("assigner not onboarded")

type PRRef struct {
	Owner    string
	Repo     string
	Index    int64
	HeadSHA  string
	Assigner string // GitFlame login of the user who assigned the bot as reviewer
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
	Token(ctx context.Context, ref PRRef) (string, error)
}
