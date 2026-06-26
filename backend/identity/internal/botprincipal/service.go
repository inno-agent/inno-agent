package botprincipal

import "context"

// Service wraps Repository with business-level operations.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// UpsertConsent links userID to gitflameUsername.
// Returns ErrUsernameTaken if another user already owns the username.
func (s *Service) UpsertConsent(ctx context.Context, userID, gitflameUsername string) error {
	return s.repo.UpsertConsent(ctx, userID, gitflameUsername)
}

// FindUserIDByGitFlameUsername returns the user_id for a gitflame_username, or
// found=false if the username has not been onboarded.
func (s *Service) FindUserIDByGitFlameUsername(ctx context.Context, gitflameUsername string) (userID string, found bool, err error) {
	return s.repo.FindUserIDByGitFlameUsername(ctx, gitflameUsername)
}
