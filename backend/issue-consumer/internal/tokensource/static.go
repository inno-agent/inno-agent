package tokensource

import (
	"context"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

var _ domain.TokenSource = (*Static)(nil)

type Static struct {
	token string
}

func NewStatic(token string) *Static {
	return &Static{token: token}
}

func (s *Static) Token(_ context.Context, _ domain.IssueRef) (string, error) {
	return s.token, nil
}
