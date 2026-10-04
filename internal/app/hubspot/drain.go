package hubspot

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

const (
	drainEvery    = 2 * time.Second
	drainBatch    = 25
	drainLease    = 3 * time.Minute
	maxJobRetries = 6
)

// retryDelays space a failing job's attempts out over several hours.
var retryDelays = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour}

// RunDrainer works the outbox until ctx ends. Safe to run in several processes.
func (s *Service) RunDrainer(ctx context.Context) {
	t := time.NewTicker(drainEvery)
	defer t.Stop()
	purge := time.NewTicker(time.Hour)
	defer purge.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-purge.C:
			_ = s.d.Repo.PurgeJobs(ctx)
		case <-t.C:
			for {
				jobs, err := s.d.Repo.ClaimJobs(ctx, drainBatch, drainLease)
				if err != nil {
					log.Warn().Err(err).Msg("hubspot: claim jobs")
					break
				}
				for i := range jobs {
					s.runJob(ctx, &jobs[i])
				}
				if len(jobs) < drainBatch || ctx.Err() != nil {
					break
				}
			}
		}
	}
}

func (s *Service) runJob(ctx context.Context, job *models.CRMSyncJob) {
	jctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	o, err := s.resolve(jctx, job.OrganizationID)
	if o == nil && err == nil {
		// The workspace left HubSpot mode: nothing to deliver to.
		_ = s.d.Repo.FailJob(ctx, job.ID, "HubSpot is no longer this workspace's CRM", nil)
		return
	}
	if err != nil {
		at := time.Now().Add(retryDelays[min(job.Attempts-1, len(retryDelays)-1)])
		if errors.Is(err, errNotConnected) || job.Attempts >= maxJobRetries {
			_ = s.d.Repo.FailJob(ctx, job.ID, "HubSpot is disconnected. Reconnect it, then retry.", nil)
			return
		}
		_ = s.d.Repo.FailJob(ctx, job.ID, "Warmbly could not reach its database; retrying.", &at)
		return
	}
	err = s.execute(jctx, o, job)
	if err == nil {
		_ = s.d.Repo.CompleteJob(ctx, job.ID)
		return
	}
	msg, retry := s.classify(jctx, o, err)
	if retry && job.Attempts < maxJobRetries {
		at := time.Now().Add(retryDelays[min(job.Attempts-1, len(retryDelays)-1)])
		if ae, ok := AsAPIError(err); ok && ae.RetryAfter > 0 {
			at = time.Now().Add(ae.RetryAfter)
		}
		_ = s.d.Repo.FailJob(ctx, job.ID, msg, &at)
		return
	}
	_ = s.d.Repo.FailJob(ctx, job.ID, msg, nil)
}

// classify words a failure for the sync health list and decides whether
// trying again could help.
func (s *Service) classify(ctx context.Context, o *org, err error) (string, bool) {
	var xe *errx.Error
	if errors.As(err, &xe) {
		return xe.Message, xe.Code == errx.ServiceUnavailable || xe.Code == errx.TooManyRequests
	}
	if ae, ok := AsAPIError(err); ok {
		ue := s.userError(ctx, o, err)
		return ue.Message, ae.Retryable() || ae.AuthProblem()
	}
	var ne net.Error
	if errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "connection") {
		return "HubSpot did not answer in time.", true
	}
	return s.userError(ctx, o, err).Message, true
}

func (s *Service) execute(ctx context.Context, o *org, job *models.CRMSyncJob) error {
	p := job.Payload
	switch job.Kind {
	case models.CRMJobLogEmail:
		return s.logEmail(ctx, o, p)
	case models.CRMJobLogEvent:
		return s.logEvent(ctx, o, p)
	case models.CRMJobLogMeeting:
		return s.logMeeting(ctx, o, p)
	case models.CRMJobPushDeal:
		return s.syncLocalDeal(ctx, o, uuidOf(p, "local_id"))
	case models.CRMJobPushTask:
		return s.syncLocalTask(ctx, o, uuidOf(p, "local_id"))
	case models.CRMJobPushNote:
		return s.syncLocalNote(ctx, o, uuidOf(p, "local_id"))
	case models.CRMJobPushContact:
		return s.pushContactFields(ctx, o, p)
	case models.CRMJobRefreshObject:
		deleted, _ := p["deleted"].(bool)
		return s.refreshObject(ctx, o, str(p, "object_type"), str(p, "external_id"), deleted)
	case models.CRMJobBackfill:
		return s.backfill(ctx, o, p)
	default:
		return nil
	}
}

func uuidOf(p map[string]any, k string) uuid.UUID {
	id, _ := uuid.Parse(str(p, k))
	return id
}

// syncLocalDeal brings HubSpot in line with a deal an automation wrote.
func (s *Service) syncLocalDeal(ctx context.Context, o *org, id uuid.UUID) error {
	deal, err := s.d.CRM.GetDeal(ctx, o.ID, id)
	if err != nil || deal == nil {
		return nil
	}
	link, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, models.CRMObjectDeal, id)
	if err != nil {
		return err
	}
	if link == nil {
		if xerr := s.PushDealCreate(ctx, o.ID, deal); xerr != nil {
			return xerr
		}
		return nil
	}
	stageExt, _, xerr := s.stageInfo(ctx, o, deal.StageID)
	if xerr != nil {
		return xerr
	}
	props := dealProperties(deal.Name, deal.Value, deal.Currency, deal.ExpectedCloseDate)
	props["dealstage"] = stageExt
	if _, err := o.Client.Update(ctx, "deals", link.ExternalID, props); err != nil {
		return err
	}
	s.notify(ctx, o.ID, contactIDString(deal.ContactID), "deal")
	return nil
}

func (s *Service) syncLocalTask(ctx context.Context, o *org, id uuid.UUID) error {
	task, err := s.d.CRM.GetCRMTask(ctx, o.ID, id)
	if err != nil || task == nil {
		return nil
	}
	link, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, models.CRMObjectTask, id)
	if err != nil {
		return err
	}
	if link != nil {
		return nil
	}
	if xerr := s.PushTaskCreate(ctx, o.ID, task); xerr != nil {
		return xerr
	}
	return nil
}

func (s *Service) syncLocalNote(ctx context.Context, o *org, id uuid.UUID) error {
	note, err := s.d.CRM.GetNote(ctx, o.ID, id)
	if err != nil || note == nil {
		return nil
	}
	if link, err := s.d.Repo.GetLinkByLocal(ctx, o.ID, models.CRMObjectNote, id); err != nil || link != nil {
		return err
	}
	if xerr := s.PushNoteCreate(ctx, o.ID, note); xerr != nil {
		return xerr
	}
	return nil
}
