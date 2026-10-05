package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (s *userService) GetUser(ctx context.Context, userID uuid.UUID) (*models.User, *errx.Error) {
	u, err := s.getUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Onboarding may have been completed by an operator or a migration.
	if u != nil && u.OnboardingCompletedAt != nil {
		return u, nil
	}

	u, xerr := s.userRepository.GetUser(ctx, userID)
	if xerr != nil {
		return nil, errx.InternalError()
	}

	if err := s.SaveUser(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}
