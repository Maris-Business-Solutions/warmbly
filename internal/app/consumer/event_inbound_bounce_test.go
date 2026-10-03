package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/advanced"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// fakeInboundAdvanced lets a handler test drive the classification the advanced
// service makes: a technical failure (non-nil errx) versus a genuine no-match
// (nil). Only the two inbound methods are overridden; the rest of the interface
// is never called on this path.
type fakeInboundAdvanced struct {
	advanced.Service
	bounceErr    *errx.Error
	complaintErr *errx.Error
}

func (f *fakeInboundAdvanced) RecordInboundBounce(context.Context, uuid.UUID, string, string, string) *errx.Error {
	return f.bounceErr
}

func (f *fakeInboundAdvanced) RecordInboundComplaint(context.Context, uuid.UUID, string, string, string) *errx.Error {
	return f.complaintErr
}

// A technical failure must be returned so the bus redelivers the event instead
// of acking a dropped bounce.
func TestHandleInboundBounceReturnsTechnicalFailure(t *testing.T) {
	s := &JobsService{AdvancedService: &fakeInboundAdvanced{bounceErr: errx.InternalError()}}
	if err := s.HandleInboundBounce(context.Background(), &models.JobEventInboundBounce{EmailID: uuid.New()}); err == nil {
		t.Fatal("a technical failure must be returned for redelivery, not swallowed")
	}
}

// A genuine no-match reads back nil and the handler acks, so the bus does not
// loop on a bounce that can never be attributed.
func TestHandleInboundBounceAcksGenuineNoMatch(t *testing.T) {
	s := &JobsService{AdvancedService: &fakeInboundAdvanced{}}
	if err := s.HandleInboundBounce(context.Background(), &models.JobEventInboundBounce{EmailID: uuid.New()}); err != nil {
		t.Fatalf("a genuine no-match must ack with nil, got %v", err)
	}
}

func TestHandleInboundComplaintReturnsTechnicalFailure(t *testing.T) {
	s := &JobsService{AdvancedService: &fakeInboundAdvanced{complaintErr: errx.InternalError()}}
	if err := s.HandleInboundComplaint(context.Background(), &models.JobEventInboundComplaint{EmailID: uuid.New()}); err == nil {
		t.Fatal("a technical failure must be returned for redelivery, not swallowed")
	}
}

func TestHandleInboundComplaintAcksGenuineNoMatch(t *testing.T) {
	s := &JobsService{AdvancedService: &fakeInboundAdvanced{}}
	if err := s.HandleInboundComplaint(context.Background(), &models.JobEventInboundComplaint{EmailID: uuid.New()}); err != nil {
		t.Fatalf("a genuine no-match must ack with nil, got %v", err)
	}
}
