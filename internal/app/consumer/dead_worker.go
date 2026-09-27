package jobs

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	workerapp "github.com/warmbly/warmbly/internal/app/worker"
	"github.com/warmbly/warmbly/internal/jobrun"
	"github.com/warmbly/warmbly/internal/models"
)

// OrgNotifier raises a permission-targeted org notification (in-app feed +
// each member's enabled channels, email digest-coalesced). Satisfied by
// *notification.Service; local interface to avoid an import cycle.
type OrgNotifier interface {
	NotifyOrg(ctx context.Context, orgID uuid.UUID, perm models.OrganizationPermission, exclude uuid.UUID, category models.NotificationCategory, title, body, link string, meta map[string]any, groupKey string)
	NotifyOrgAboutMessage(ctx context.Context, orgID uuid.UUID, perm models.OrganizationPermission, uniboxEmailID uuid.UUID, category models.NotificationCategory, title, body, link string, meta map[string]any, groupKey string)
}

// OperatorNotifier is the instance-wide operator alert surface, declared here
// so this package needs no import of it. Nil disables it.
type OperatorNotifier interface {
	NotifyOperator(key, title, summary string, fields map[string]string)
}

type workerRecoveryOutcome struct {
	Total          int
	Reassigned     int
	Stranded       int
	FailureReasons map[string]int
}

func (o workerRecoveryOutcome) state() string {
	switch {
	case o.Stranded == 0:
		return "complete"
	case o.Reassigned > 0:
		return "partial"
	default:
		return "failed"
	}
}

func (o workerRecoveryOutcome) failureSummary() string {
	if len(o.FailureReasons) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(o.FailureReasons))
	for reason := range o.FailureReasons {
		keys = append(keys, reason)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, reason := range keys {
		parts = append(parts, fmt.Sprintf("%s: %d", strings.ReplaceAll(reason, "_", " "), o.FailureReasons[reason]))
	}
	return strings.Join(parts, ", ")
}

type orgRecoveryOutcome struct {
	Reassigned int
	Stranded   int
}

// notifyOperatorWorkerDown alerts the operator with the recovery result.
func (s *JobsService) notifyOperatorWorkerDown(ctx context.Context, workerID uuid.UUID, outcome workerRecoveryOutcome) {
	if s.OpsNotifier == nil || s.Cache == nil {
		return
	}
	ok, err := s.Cache.SetNX(ctx, "worker:opsnotify:"+workerID.String(), "1", 6*time.Hour).Result()
	if err != nil || !ok {
		return
	}
	summary := "All affected mailboxes were moved to eligible live workers."
	if outcome.state() == "partial" {
		summary = "Some affected mailboxes were moved; the remaining mailboxes are paused."
	} else if outcome.state() == "failed" {
		summary = "Recovery failed; affected mailboxes remain paused."
	}
	s.OpsNotifier.NotifyOperator(
		"worker.offline",
		"Worker stopped responding",
		summary,
		map[string]string{
			"Worker":          workerID.String(),
			"Outcome":         outcome.state(),
			"Mailboxes":       strconv.Itoa(outcome.Total),
			"Reassigned":      strconv.Itoa(outcome.Reassigned),
			"Stranded":        strconv.Itoa(outcome.Stranded),
			"Failure reasons": outcome.failureSummary(),
		},
	)
}

// notifyWorkerDown tells each affected org's manage_emails members about a
// dead worker, at most once per org and worker incident: cloud workers carry
// mailboxes for many orgs, and one org's alert must not suppress another's if
// reassignment completes across multiple scans. The shared group key coalesces
// an org's recipients into one email with everyone in To.
func (s *JobsService) notifyWorkerDown(ctx context.Context, workerID uuid.UUID, orgs map[uuid.UUID]orgRecoveryOutcome) {
	if s.Notifier == nil || len(orgs) == 0 {
		return
	}
	for orgID, outcome := range orgs {
		key := "worker:downnotify:" + workerID.String() + ":" + orgID.String()
		ok, err := s.Cache.SetNX(ctx, key, "1", 6*time.Hour).Result()
		if err != nil || !ok {
			continue
		}
		total := outcome.Reassigned + outcome.Stranded
		noun := fmt.Sprintf("%d of your mailboxes were", total)
		if total == 1 {
			noun = "One of your mailboxes was"
		}
		body := noun + " on a sending worker that stopped responding. "
		switch {
		case outcome.Stranded == 0:
			body += "They were moved to a healthy worker automatically; no action is needed."
			if total == 1 {
				body = noun + " on a sending worker that stopped responding. It was moved to a healthy worker automatically; no action is needed."
			}
		case outcome.Reassigned == 0:
			body += "Sending from them is paused until a replacement worker is available."
		default:
			body += fmt.Sprintf("%d were moved automatically; sending from the remaining %d is paused.", outcome.Reassigned, outcome.Stranded)
		}
		meta := map[string]any{"reassigned": outcome.Reassigned, "stranded": outcome.Stranded}
		s.Notifier.NotifyOrg(ctx, orgID, models.PermManageEmails, uuid.Nil, models.NotifWorkerDowntime,
			"Sending worker went offline", body, "/app/emails", meta,
			"worker_down:"+workerID.String())
	}
}

// StartDeadWorkerDetection periodically checks for workers whose heartbeat has
// expired and reassigns their email accounts to healthy workers.
// Runs every interval until the context is cancelled.
func (s *JobsService) StartDeadWorkerDetection(ctx context.Context, interval time.Duration) {
	if s.WorkerRepo == nil {
		return
	}
	jobrun.Loop(ctx, "dead_worker_detection", interval, false, func(ctx context.Context) error {
		detectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		s.detectDeadWorkers(detectCtx)
		return nil
	})
}

func (s *JobsService) detectDeadWorkers(ctx context.Context) {
	// Get all workers from the database
	workers, err := s.WorkerRepo.GetAllActiveWorkers(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("dead worker detection: failed to list workers")
		return
	}

	for _, w := range workers {
		key := fmt.Sprintf("worker:heartbeat:%s", w.ID.String())
		exists, err := s.Cache.Exists(ctx, key).Result()
		if err != nil {
			continue
		}

		if exists > 0 {
			continue // Worker is alive
		}

		// Worker heartbeat expired - mark as stale and reassign emails
		log.Warn().Str("worker_id", w.ID.String()).Msg("dead worker detected - heartbeat expired")

		// A restart is not a death. The heartbeat key lives 3 minutes and an
		// auto-update replaces the container inside that, so evacuating on the
		// key alone moved 92 mailboxes off a worker that was back seconds
		// later (#583). Moving a mailbox changes the address its provider sees
		// and buys a sign-in challenge, so the bar is the same one
		// deactivateIfLongDead already applies to merely retiring the row.
		if !s.unreachableLongEnoughToEvacuate(ctx, w) {
			continue
		}

		// Get all email accounts assigned to this worker
		accountIDs, err := s.WorkerRepo.GetEmailAccountsByWorkerID(ctx, w.ID)
		if err != nil {
			log.Error().Err(err).Str("worker_id", w.ID.String()).Msg("failed to get accounts for dead worker")
			continue
		}

		if len(accountIDs) == 0 {
			// Nothing to move: retire the row so it stops being rescanned
			// every interval. Churned ids without WORKER_ID accumulate here
			// forever otherwise; a returning worker reactivates itself on its
			// next heartbeat upsert.
			s.deactivateIfLongDead(ctx, w)
			continue
		}

		// Score each mailbox independently so recovery does not create a hotspot.
		outcome := workerRecoveryOutcome{Total: len(accountIDs), FailureReasons: map[string]int{}}
		affectedOrgs := map[uuid.UUID]orgRecoveryOutcome{}
		destinations := map[uuid.UUID]struct{}{}
		recordFailure := func(reason string, orgID *uuid.UUID) {
			outcome.Stranded++
			outcome.FailureReasons[reason]++
			if orgID != nil {
				orgOutcome := affectedOrgs[*orgID]
				orgOutcome.Stranded++
				affectedOrgs[*orgID] = orgOutcome
			}
		}
		for _, accountID := range accountIDs {
			account, aerr := s.EmailRepository.GetByID(ctx, accountID)
			if aerr != nil || account == nil {
				log.Warn().Err(aerr).Str("account_id", accountID.String()).Msg("dead worker reassign: mailbox owner unavailable")
				recordFailure("mailbox_lookup_failed", nil)
				continue
			}
			if account.OrganizationID == nil {
				log.Warn().Str("account_id", accountID.String()).Msg("dead worker reassign: mailbox has no organization")
				recordFailure("organization_missing", nil)
				continue
			}

			var target *models.Worker
			if s.AssignmentService != nil {
				result, rerr := s.AssignmentService.SelectWorkerFor(ctx, workerapp.PlacementLookup{
					EmailAccountID:  accountID,
					OrgID:           *account.OrganizationID,
					CurrentWorkerID: &w.ID,
					ExcludeWorkerID: &w.ID,
					Region:          w.Region,
				})
				if rerr != nil {
					log.Warn().Err(rerr).Str("account_id", accountID.String()).Msg("dead worker reassign: placement failed")
					recordFailure("placement_failed", account.OrganizationID)
					continue
				}
				if result != nil {
					target = result.Worker
				}
			} else {
				var ferr error
				target, ferr = s.findHealthyWorker(ctx, w)
				if ferr != nil {
					log.Warn().Err(ferr).Str("account_id", accountID.String()).Msg("dead worker reassign: fallback lookup failed")
					recordFailure("replacement_lookup_failed", account.OrganizationID)
					continue
				}
			}
			if target == nil {
				recordFailure("no_eligible_worker", account.OrganizationID)
				continue
			}

			if s.AssignmentService != nil {
				if err := s.AssignmentService.MoveMailbox(ctx, accountID, &w.ID, target.ID); err != nil {
					log.Error().Err(err).Str("account_id", accountID.String()).Msg("failed to reassign email account")
					recordFailure("move_failed", account.OrganizationID)
					continue
				}
			} else if err := s.WorkerRepo.MoveEmailAccountWorker(
				ctx,
				accountID,
				&w.ID,
				target.ID,
				workerapp.MailboxWeight(account.Provider, account.Warmup != nil),
			); err != nil {
				log.Error().Err(err).Str("account_id", accountID.String()).Msg("failed to reassign email account")
				recordFailure("move_failed", account.OrganizationID)
				continue
			}

			outcome.Reassigned++
			orgOutcome := affectedOrgs[*account.OrganizationID]
			orgOutcome.Reassigned++
			affectedOrgs[*account.OrganizationID] = orgOutcome
			destinations[target.ID] = struct{}{}

			// The backend's worker reconciler loads the account onto its new
			// worker with the full payload (decrypted credentials, cursors,
			// sync policy). Publishing a bare ADD_EMAIL from here only made
			// the worker reject it and log an error per mailbox.
		}

		if outcome.Reassigned > 0 {
			log.Info().
				Str("dead_worker", w.ID.String()).
				Int("destinations", len(destinations)).
				Int("reassigned", outcome.Reassigned).
				Int("stranded", outcome.Stranded).
				Msg("email accounts reassigned from dead worker")
		} else {
			log.Warn().Str("worker_id", w.ID.String()).Int("stranded", outcome.Stranded).Msg("dead worker recovery failed")
		}

		// uuid.Nil identifies this as a system action in the operator audit.
		if s.AdminRepo != nil {
			_ = s.AdminRepo.CreateAuditLog(ctx, &models.AdminAuditLog{
				ID:          uuid.New(),
				AdminUserID: uuid.Nil,
				Action:      "auto_reassign",
				TargetType:  "worker",
				TargetID:    w.ID,
				Details: map[string]any{
					"outcome":             outcome.state(),
					"replacement_workers": len(destinations),
					"accounts_reassigned": outcome.Reassigned,
					"accounts_stranded":   outcome.Stranded,
					"failure_reasons":     outcome.FailureReasons,
					"reason":              "heartbeat_expired",
				},
				IPAddress: "",
				UserAgent: "system",
				CreatedAt: time.Now(),
			})
		}

		s.notifyWorkerDown(ctx, w.ID, affectedOrgs)
		s.notifyOperatorWorkerDown(ctx, w.ID, outcome)

		if outcome.Reassigned == len(accountIDs) {
			s.deactivateIfLongDead(ctx, w)
		}
	}
}

// MailboxEvacuationGrace is how long a worker has to be unreachable before its
// mailboxes are moved. It spans a container replacement (pull, stop, start,
// boot) with room to spare, so a version rollout costs no migrations.
const MailboxEvacuationGrace = 10 * time.Minute

// unreachableLongEnoughToEvacuate reports whether a worker with no heartbeat
// key has also been absent from the registry long enough to be worth moving
// mailboxes off.
//
// Both signals are required for the same reason deactivateIfLongDead needs
// both: during a Redis outage every heartbeat key vanishes at once while
// POSTed beats keep last_seen_at fresh, and evacuating on the key alone would
// migrate every mailbox in the fleet at once.
//
// The worker is re-read because the caller's copy is a snapshot from the top of
// a scan that walks the whole fleet.
func (s *JobsService) unreachableLongEnoughToEvacuate(ctx context.Context, w models.Worker) bool {
	current, err := s.WorkerRepo.GetByID(ctx, w.ID)
	if err != nil || current == nil {
		return false
	}
	// Never seen at all means the age is unknown, not old: a worker that has
	// only just registered has no mailboxes worth moving anyway.
	if current.LastSeenAt == nil {
		return false
	}
	if time.Since(*current.LastSeenAt) < MailboxEvacuationGrace {
		log.Info().
			Str("worker_id", w.ID.String()).
			Time("last_seen_at", *current.LastSeenAt).
			Msg("worker is unreachable but within the evacuation grace; leaving its mailboxes in place")
		return false
	}
	// One last heartbeat check against the freshly-read row, as
	// deactivateIfLongDead does: the key was read at the top of the scan, and
	// a worker that came back in between must not lose its mailboxes to a
	// stale last_seen_at.
	if s.Cache != nil {
		if n, herr := s.Cache.Exists(ctx, "worker:heartbeat:"+w.ID.String()).Result(); herr != nil || n > 0 {
			return false
		}
	}
	return true
}

// deactivateIfLongDead retires a heartbeat-expired worker row, but only when
// its registry timestamp is stale too. During a Redis outage every heartbeat
// key vanishes at once while POSTed beats keep last_seen_at fresh; gating on
// both signals keeps that from deactivating the whole live fleet.
//
// The worker is re-read first because the caller's copy is a snapshot from the
// top of the scan, which walks the whole fleet and can take a while: a worker
// that booted during the scan must not be deactivated on stale evidence.
func (s *JobsService) deactivateIfLongDead(ctx context.Context, w models.Worker) {
	current, err := s.WorkerRepo.GetByID(ctx, w.ID)
	if err != nil || current == nil {
		return
	}
	// Never seen at all means the age is unknown, not old. Leave it alone
	// rather than retiring a row that may be mid-registration.
	if current.LastSeenAt == nil || time.Since(*current.LastSeenAt) < 10*time.Minute {
		return
	}
	// One last heartbeat check against the freshly-read row.
	if n, herr := s.Cache.Exists(ctx, "worker:heartbeat:"+w.ID.String()).Result(); herr != nil || n > 0 {
		return
	}
	if err := s.FleetNodeRepo.Deactivate(ctx, w.ID); err != nil {
		log.Warn().Err(err).Str("worker_id", w.ID.String()).Msg("failed to deactivate dead worker")
		return
	}
	log.Info().Str("worker_id", w.ID.String()).Msg("dead worker deactivated")
}

func (s *JobsService) findHealthyWorker(ctx context.Context, deadWorker models.Worker) (*models.Worker, error) {
	workers, err := s.WorkerRepo.ListPlaceableWorkers(ctx)
	if err != nil {
		return nil, err
	}

	for _, w := range workers {
		if w.ID == deadWorker.ID {
			continue
		}
		// Check if this worker is alive
		key := fmt.Sprintf("worker:heartbeat:%s", w.ID.String())
		exists, err := s.Cache.Exists(ctx, key).Result()
		if err != nil || exists == 0 {
			continue
		}
		return &w, nil
	}

	return nil, nil
}
