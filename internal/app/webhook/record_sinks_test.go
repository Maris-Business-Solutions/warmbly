package webhook

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// Two connected CRMs each keep their own record of every event, so wiring a
// second record sink must add it, not replace the first.
func TestWireRecordSinkKeepsEverySink(t *testing.T) {
	s := &service{}
	var got []string
	s.WireRecordSink(func(context.Context, uuid.UUID, models.WebhookEventType, any) { got = append(got, "hubspot") })
	s.WireRecordSink(func(context.Context, uuid.UUID, models.WebhookEventType, any) { got = append(got, "salesforce") })
	s.WireRecordSink(nil)
	for _, sink := range s.recordSinks {
		sink(context.Background(), uuid.New(), models.WebhookEventCampaignEmailSent, nil)
	}
	if len(got) != 2 || got[0] != "hubspot" || got[1] != "salesforce" {
		t.Fatalf("every sink sees the event once: %v", got)
	}
}
