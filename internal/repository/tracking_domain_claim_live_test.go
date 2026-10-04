package repository

import (
	"context"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/errx"
)

// A tracking host one workspace has verified cannot be saved, applied or
// verified by another, while the holder keeps re-verifying it freely.
func TestLiveTrackingDomainBelongsToOneOrganization(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	a := newUniboxFolderFixture(t, handle.Pool)
	b := newUniboxFolderFixture(t, handle.Pool)
	repo := NewEmailRepostory(handle, nil)
	ctx := context.Background()
	host := "link-" + a.org.String()[:8] + ".example.test"
	now := time.Now().UTC()

	if xerr := repo.UpdateTrackingDomain(ctx, a.org.String(), a.mailbox.String(), host, true, &now); xerr != nil {
		t.Fatalf("holder verifies: %v", xerr)
	}
	if xerr := repo.UpdateTrackingDomain(ctx, a.org.String(), a.mailbox.String(), host, true, &now); xerr != nil {
		t.Fatalf("holder re-verifies: %v", xerr)
	}
	if xerr := repo.UpdateTrackingDomain(ctx, b.org.String(), b.mailbox.String(), host, false, nil); xerr != errx.ErrTrackingDomainTaken {
		t.Fatalf("another workspace saved the host: %v", xerr)
	}
	if _, xerr := repo.SetDomainTracking(ctx, b.org, "test.local", host, true, &now); xerr != errx.ErrTrackingDomainTaken {
		t.Fatalf("another workspace applied the host: %v", xerr)
	}

	// The sweep path writes no domain, so the trigger is what refuses it.
	if _, err := handle.Pool.Exec(ctx, `UPDATE email_accounts SET tracking_domain = $2 WHERE id = $1`, b.mailbox, host); err != nil {
		t.Fatalf("seed unverified host: %v", err)
	}
	if xerr := repo.SetTrackingDomainVerified(ctx, b.mailbox, true, &now); xerr != errx.ErrTrackingDomainTaken {
		t.Fatalf("the sweep verified a host another workspace holds: %v", xerr)
	}
}
