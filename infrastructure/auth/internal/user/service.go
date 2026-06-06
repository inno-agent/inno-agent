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

func (s *Service) GetContext(ctx context.Context, userID string) (UserContext, error) {
	return s.repo.GetContext(ctx, userID)
}

func (s *Service) UpdateContext(ctx context.Context, userID string, data []byte) error {
	return s.repo.UpdateContext(ctx, userID, data)
}
