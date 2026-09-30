package tasks

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/events"
	"github.com/warmbly/warmbly/internal/models"
)

type capturingPublisher struct {
	events.Publisher
	params *events.SendEmailParams
}

func (p *capturingPublisher) PublishSendEmail(_ context.Context, _ uuid.UUID, params *events.SendEmailParams) error {
	p.params = params
	return nil
}

func TestSendCarriesReplyToOnlyOutsideWarmup(t *testing.T) {
	worker, org := uuid.New(), uuid.New()
	account := models.Email{ID: uuid.New(), OrganizationID: &org, WorkerID: &worker, Email: "b@acme.com", ReplyTo: "a@acme.com"}
	for _, tt := range []struct {
		name   string
		warmup bool
		want   string
	}{
		{name: "campaign", want: "a@acme.com"},
		// Warmup verification reads replies back in the sending mailbox.
		{name: "warmup", warmup: true, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pub := &capturingPublisher{}
			sender := NewEmailSender(nil, pub)
			if err := sender.Send(context.Background(), uuid.New(), EmailMessage{To: []string{"x@y.test"}, IsWarmup: tt.warmup}, account); err != nil {
				t.Fatal(err)
			}
			if pub.params.ReplyTo != tt.want {
				t.Fatalf("ReplyTo = %q, want %q", pub.params.ReplyTo, tt.want)
			}
		})
	}
}
