package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

type capturedOrgNotice struct {
	orgID    uuid.UUID
	perm     models.OrganizationPermission
	message  uuid.UUID
	category models.NotificationCategory
	title    string
	body     string
	link     string
	groupKey string
}

type captureOrgNotifier struct{ got []capturedOrgNotice }

func (c *captureOrgNotifier) NotifyOrg(context.Context, uuid.UUID, models.OrganizationPermission, uuid.UUID, models.NotificationCategory, string, string, string, map[string]any, string) {
}

func (c *captureOrgNotifier) NotifyOrgAboutMessage(_ context.Context, orgID uuid.UUID, perm models.OrganizationPermission, message uuid.UUID, category models.NotificationCategory, title, body, link string, _ map[string]any, groupKey string) {
	c.got = append(c.got, capturedOrgNotice{orgID, perm, message, category, title, body, link, groupKey})
}

// The notice goes to members who both keep mailboxes running and can open the
// message, and is tied to the message so reading it reads the notification.
func TestNotifyActionRequiredReachesMailboxManagers(t *testing.T) {
	n := &captureOrgNotifier{}
	s := &JobsService{Notifier: n}
	orgID := uuid.New()
	msg := &models.EmailMessageStoreData{
		ID:       uuid.New(),
		EmailID:  uuid.New(),
		ThreadID: "thread/1",
		Subject:  "  Payment declined  ",
	}

	s.notifyActionRequired(context.Background(), orgID, msg)

	if len(n.got) != 1 {
		t.Fatalf("raised %d notifications, want 1", len(n.got))
	}
	got := n.got[0]
	if got.orgID != orgID || got.message != msg.ID || got.category != models.NotifInboxActionRequired {
		t.Fatalf("notice = %+v", got)
	}
	if got.perm != models.PermManageEmails|models.PermAccessUnibox {
		t.Fatalf("perm = %b, want manage mailboxes and use the inbox", got.perm)
	}
	if got.body != "Payment declined" || got.link != "/app/unibox/all/thread%2F1" || got.groupKey == "" {
		t.Fatalf("notice = %+v", got)
	}
}

func TestNotifyActionRequiredWithoutNotifierIsQuiet(t *testing.T) {
	s := &JobsService{}
	s.notifyActionRequired(context.Background(), uuid.New(), &models.EmailMessageStoreData{ID: uuid.New()})
}
