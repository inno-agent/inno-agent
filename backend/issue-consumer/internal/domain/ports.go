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

type IssueRef struct {
	Owner         string
	Repo          string
	Index         int64
	Assigner      string
	Title         string
	Body          string
	IssueType     string
	DefaultBranch string
}

type GeneratedFile struct {
	Path    string
	Content string
}

type GenerationResult struct {
	Files   []GeneratedFile
	Summary string
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMProvider interface {
	Chat(ctx context.Context, messages []LLMMessage, modelName string) (string, error)
}

type IssueSource interface {
	GetIssue(ctx context.Context, ref IssueRef) (title, body string, err error)
	GetRawFile(ctx context.Context, ref IssueRef, path string) (content string, found bool, err error)
}

type CodePusher interface {
	PushFiles(ctx context.Context, ref IssueRef, branch string, files []GeneratedFile, message string) error
}

type CommentPoster interface {
	PostIssueComment(ctx context.Context, ref IssueRef, body string) error
}

type Generator interface {
	Generate(ctx context.Context, ref IssueRef) (*GenerationResult, error)
}

type TokenSource interface {
	Token(ctx context.Context, ref IssueRef) (token string, err error)
}
