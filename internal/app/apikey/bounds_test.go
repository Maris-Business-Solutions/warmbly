package apikey

import (
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

func TestWithinKeyBoundsAKeyByItsCreator(t *testing.T) {
	box := uuid.New()
	caller := &models.APIKey{
		Permissions:          models.APIPermReadContacts | models.APIPermAPIKeys,
		AllowedIPs:           []string{"10.0.0.0/8", "192.0.2.7"},
		AllowedEmailAccounts: []uuid.UUID{box},
	}
	ok := func(perms uint64, ips []string, accounts []uuid.UUID) bool {
		return WithinKey(caller, perms, ips, accounts) == nil
	}

	if !ok(models.APIPermReadContacts, []string{"10.1.0.0/16", "192.0.2.7"}, []uuid.UUID{box}) {
		t.Error("a narrower key was refused")
	}
	if ok(models.APIPermReadContacts|models.APIPermWriteContacts, []string{"10.1.2.3"}, []uuid.UUID{box}) {
		t.Error("a key with an extra permission was accepted")
	}
	if ok(models.APIPermReadContacts, []string{"10.1.2.3"}, nil) {
		t.Error("a key reaching every mailbox was accepted")
	}
	if ok(models.APIPermReadContacts, []string{"10.1.2.3"}, []uuid.UUID{uuid.New()}) {
		t.Error("a key reaching another mailbox was accepted")
	}
	if ok(models.APIPermReadContacts, nil, []uuid.UUID{box}) {
		t.Error("a key allowed from anywhere was accepted")
	}
	if ok(models.APIPermReadContacts, []string{"0.0.0.0/0"}, []uuid.UUID{box}) {
		t.Error("a wider block was accepted")
	}
	if ok(models.APIPermReadContacts, []string{"192.0.2.8"}, []uuid.UUID{box}) {
		t.Error("another address was accepted")
	}

	open := &models.APIKey{Permissions: models.APIPermReadContacts}
	if WithinKey(open, models.APIPermReadContacts, nil, nil) != nil {
		t.Error("an unrestricted key could not mint an unrestricted key with its own permissions")
	}
	if WithinKey(nil, 0, nil, nil) == nil {
		t.Error("a missing caller key was accepted")
	}
}
