package user

import "context"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UpsertIdentity(ctx context.Context, provider, sub, email string) (User, error) {
	return s.repo.UpsertIdentity(ctx, provider, sub, email)
}
