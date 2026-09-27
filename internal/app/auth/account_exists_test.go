package auth

import (
	"context"
	"net/mail"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// takenUserRepo refuses every CreateUser as a duplicate address; lookups find
// the account only after the first one, as if a concurrent signup created it.
type takenUserRepo struct {
	repository.UserRepository
	existing *models.User
	lookups  int
}

func (r *takenUserRepo) CreateUser(context.Context, *mail.Address, string) (*models.User, error) {
	return nil, repository.ErrUserEmailTaken
}

func (r *takenUserRepo) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	r.lookups++
	if r.lookups > 1 && r.existing != nil && r.existing.Email == email {
		return r.existing, nil
	}
	return nil, errx.ErrUser
}

func TestCreateAccountAnswersAnExistingAddressWithAConflict(t *testing.T) {
	svc := &authService{userRepository: &takenUserRepo{}, userService: noopUserService{}}

	_, err := svc.createAccount(context.Background(), "ada@acme.com", "hash", SignupAttribution{}, SignupOrigin{})
	if err != errx.ErrAccountExists {
		t.Fatalf("err = %v, want ErrAccountExists", err)
	}
}

// Losing the provisioning race resolves against the account that won, on the
// same terms as any existing account: a password account still waits for its password.
func TestFederatedProvisioningRaceResolvesAgainstTheWinner(t *testing.T) {
	winner := &models.User{ID: uuid.New(), Email: "owner@example.com"}
	ids := &memoryIdentities{known: map[string]uuid.UUID{}}
	s := &authService{
		userRepository: &takenUserRepo{existing: winner},
		authRepository: &hashAuthRepo{hash: "$argon2id$hash"},
		identities:     ids,
		policy:         &config.AuthPolicy{SSOAutoProvision: true},
	}

	res := resolve(t, s)
	if res.UserID != winner.ID {
		t.Fatalf("resolved %s, want %s", res.UserID, winner.ID)
	}
	if !res.LinkRequired {
		t.Fatal("a password account was linked on the address alone")
	}
	if len(ids.linked) != 0 {
		t.Fatalf("linked %v before the password arrived", ids.linked)
	}
}
