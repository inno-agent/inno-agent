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
	Creator       string
	Title         string
	Body          string
	IssueType     string
	DefaultBranch string
}

type ChangedFile struct {
	Path   string
	Status string
}

type GenerationResult struct {
	// Branch is the name the agent already pushed to — issue-consumer no
	// longer pushes anything itself, it only creates the PR from this.
	Branch       string
	ChangedFiles []ChangedFile
	Summary      string
	// Verified reports whether the agent's build/test verification passed before
	// the code was pushed. False means the change was pushed with a warning.
	Verified bool
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

type PullRequestCreator interface {
	CreatePullRequest(ctx context.Context, ref IssueRef, headBranch, title, body string, reviewers []string) (int64, error)
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
