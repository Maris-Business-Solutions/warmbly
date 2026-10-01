package poollink

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/email"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/utils/validate"
)

// settingsEmails applies the write's own time validation and records what passed.
type settingsEmails struct {
	email.EmailService
	got *models.UpdateEmail
}

func (e *settingsEmails) Update(_ context.Context, _, _, _ string, u *models.UpdateEmail) (*models.Email, *errx.Error) {
	for _, v := range []*string{u.WarmupStartTime, u.WarmupEndTime} {
		if v != nil {
			if xerr := validate.CampaignTime(*v); xerr != nil {
				return nil, xerr
			}
		}
	}
	e.got = u
	return &models.Email{}, nil
}

// An instance reads its time columns as "08:00:00.000000"; the ramp must still land on the cloud.
func TestApplyWarmupSettingsAcceptsInstanceClockFormat(t *testing.T) {
	emails := &settingsEmails{}
	s := &service{emailSvc: emails}
	s.applyWarmupSettings(context.Background(), uuid.New(), uuid.NewString(), uuid.New(), models.PoolLinkWarmupSettings{
		Base: 5, Max: 30, StartTime: "08:00:00.000000", EndTime: "17:30:00", Timezone: "America/New_York",
	})
	if emails.got == nil {
		t.Fatal("warmup settings were refused; the mailbox would warm on defaults")
	}
	if emails.got.WarmupStartTime == nil || *emails.got.WarmupStartTime != "08:00" {
		t.Fatalf("start = %v, want 08:00", emails.got.WarmupStartTime)
	}
	if emails.got.WarmupEndTime == nil || *emails.got.WarmupEndTime != "17:30" {
		t.Fatalf("end = %v, want 17:30", emails.got.WarmupEndTime)
	}
	if emails.got.WarmupBase == nil || *emails.got.WarmupBase != 5 {
		t.Fatalf("base = %v, want 5", emails.got.WarmupBase)
	}
}

// A cleartext credential for a remote server can never load on the cloud, so enrollment refuses it.
func TestCloudReachableRefusesRemoteCleartext(t *testing.T) {
	tls := &models.Service{Host: "mail.example.test", Port: 993, Security: models.MailSecurityTLS}
	none := &models.Service{Host: "mail.example.test", Port: 25, Security: models.MailSecurityNone}
	if xerr := cloudReachable(&models.SmtpImap{SMTP: none, IMAP: tls}); xerr != ErrCleartextMailbox {
		t.Fatalf("cleartext SMTP: got %v, want %v", xerr, ErrCleartextMailbox)
	}
	if xerr := cloudReachable(&models.SmtpImap{SMTP: tls, IMAP: tls}); xerr != nil {
		t.Fatalf("TLS on both legs: got %v, want nil", xerr)
	}
}
