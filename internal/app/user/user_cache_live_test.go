package user

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/infrastructure/cache"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type onboardedUserRepo struct {
	repository.UserRepository
	user  *models.User
	reads int
}

func (r *onboardedUserRepo) GetUser(context.Context, uuid.UUID) (*models.User, error) {
	r.reads++
	return r.user, nil
}

func TestLiveUserRefreshesCachedOnboardingAfterBackfill(t *testing.T) {
	url := os.Getenv("WARMBLY_TEST_REDIS")
	if url == "" {
		t.Skip("WARMBLY_TEST_REDIS not set")
	}
	store, err := cache.New(url)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	userID := uuid.New()
	t.Cleanup(func() { store.Del(ctx, CacheKey(userID)) })
	now := time.Now()
	repo := &onboardedUserRepo{user: &models.User{ID: userID, OnboardingCompletedAt: &now}}
	svc := &userService{cache: store, userRepository: repo}
	if xerr := svc.SaveUser(ctx, &models.User{ID: userID}); xerr != nil {
		t.Fatal(xerr)
	}
	for range 2 {
		u, xerr := svc.GetUser(ctx, userID)
		if xerr != nil || u.OnboardingCompletedAt == nil {
			t.Fatalf("cached onboarding was not refreshed: %v", xerr)
		}
	}
	if repo.reads != 1 {
		t.Fatalf("onboarded users should still be cached, repository reads=%d", repo.reads)
	}
}
