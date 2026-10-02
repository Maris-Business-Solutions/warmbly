package inboxtag

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/repository"
)

const (
	// DefaultFollowUpPageSize is how many messages one page of the sweep reads.
	DefaultFollowUpPageSize = 500
	// followUpFailureLimit is how many consecutive failures at one place the sweep takes before stepping past it.
	followUpFailureLimit = 3
	// followUpSettle keeps the changed-thread check behind writes that may still be committing.
	followUpSettle = time.Minute
	// followUpLeaseMargin is how long a walker's lease outlives its budget.
	followUpLeaseMargin = 2 * time.Minute
	// followUpUnboundedLease is the lease of a pass with no budget, renewed on every save.
	followUpUnboundedLease = 10 * time.Minute
)

// maxRowID orders after every row id, so a mark at (t, maxRowID) covers everything at t.
var maxRowID = uuid.UUID{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// errLeaseLost stops a walker whose lease another walker has taken over.
var errLeaseLost = errors.New("follow-up sweep: lease lost")

// FollowUpSweep bounds one pass of the follow-up sweep.
type FollowUpSweep struct {
	// Since is the oldest activity a thread may have and still be swept.
	Since time.Time
	// Fresh is the furthest back the check of changed threads looks, its first run included; zero skips it.
	Fresh time.Duration
	// Budget stops a pass from starting another page; zero runs the cycle to its end.
	Budget time.Duration
	// PageSize is how many messages a page reads; zero is DefaultFollowUpPageSize.
	PageSize int
	// Full sweeps every thread from the newest and leaves the scheduled sweep's state alone.
	Full bool
}

// FollowUpProgress reports what one sweep changed.
type FollowUpProgress struct {
	Threads  int
	Labelled map[string]int
	Cleared  int
	Pages    int
	// Skipped counts pages and threads stepped past after repeated failures.
	Skipped int
	// Complete is a cycle that reached its end, so the next pass starts at the newest.
	Complete bool
	// Busy is a pass that found another walker holding the workspace.
	Busy bool
}

// SweepFollowUps recomputes follow-up labels a page at a time, resuming the workspace's cycle; it makes no model calls.
func (s *Service) SweepFollowUps(ctx context.Context, orgID uuid.UUID, opts FollowUpSweep) (FollowUpProgress, error) {
	p := FollowUpProgress{Labelled: map[string]int{}}
	if s == nil || s.repo == nil || s.categories == nil {
		return p, nil
	}
	if opts.PageSize <= 0 {
		opts.PageSize = DefaultFollowUpPageSize
	}
	mailboxes, err := s.repo.FollowUpMailboxes(ctx, orgID)
	if err != nil {
		return p, err
	}

	// The hourly sweep is also how a workspace that predates the feature
	// gets its labels: the whole taxonomy when classification is on, the
	// follow-up labels otherwise. Idempotent and cached, so it costs nothing
	// after the first pass.
	seed := append([]string{}, SeedSet()...)
	if s.Enabled() {
		seed = append(seed, CustomLabels(s.workspace(ctx, orgID).questions)...)
	}
	if err := s.categories.EnsureAll(ctx, orgID, seed); err != nil {
		log.Warn().Err(err).Msg("inbox tagging: could not seed labels")
	}

	now := time.Now()
	w := &followUpWalk{s: s, orgID: orgID, opts: opts, started: now, now: now, seen: map[string]bool{}, p: &p}
	if opts.Full {
		complete, err := w.walk(ctx, mailboxes, nil)
		p.Complete = complete && err == nil
		return p, err
	}

	w.owner = uuid.New()
	w.lease = followUpUnboundedLease
	if opts.Budget > 0 {
		w.lease = opts.Budget + followUpLeaseMargin
	}
	st, err := s.repo.ClaimFollowUpSweep(ctx, orgID, w.owner, w.lease)
	if err != nil {
		return p, err
	}
	if st == nil {
		p.Busy = true
		return p, nil
	}
	w.state = st
	defer w.release(ctx)

	err = w.run(ctx, mailboxes)
	if errors.Is(err, errLeaseLost) {
		p.Busy = true
		return p, nil
	}
	return p, err
}

type followUpWalk struct {
	s       *Service
	orgID   uuid.UUID
	opts    FollowUpSweep
	started time.Time
	now     time.Time
	seen    map[string]bool
	p       *FollowUpProgress

	// owner, lease and state are nil-valued on a Full sweep, which persists nothing.
	owner uuid.UUID
	lease time.Duration
	state *repository.FollowUpSweepState
	// progressed is set once the walk moves past where its last failure was counted.
	progressed bool
}

func (w *followUpWalk) run(ctx context.Context, mailboxes []uuid.UUID) error {
	if w.opts.Fresh > 0 {
		if err := w.fresh(ctx); err != nil {
			if errors.Is(err, errLeaseLost) || ctx.Err() != nil {
				return err
			}
			log.Warn().Err(err).Str("org_id", w.orgID.String()).Msg("inbox tagging: changed threads not checked this pass")
		}
	}
	complete, err := w.walk(ctx, mailboxes, w.state.Cursor)
	if err != nil || !complete {
		return err
	}
	w.state.Cursor = nil
	w.state.PageFailures = 0
	if err := w.save(ctx); err != nil {
		return err
	}
	w.p.Complete = true
	return nil
}

// fresh checks every thread with a message stored or a verdict written since the last check.
func (w *followUpWalk) fresh(ctx context.Context) error {
	st := w.state
	mark := repository.FollowUpMark{At: st.Now.Add(-w.opts.Fresh)}
	if st.Fresh != nil && st.Fresh.At.After(mark.At) {
		mark = *st.Fresh
	}
	until := st.Now.Add(-followUpSettle)
	if !mark.At.Before(until) {
		return nil
	}
	progressed := false
	for first := true; ; first = false {
		if !first && w.spent() {
			return nil
		}
		changes, err := w.s.repo.FollowUpChanges(ctx, w.orgID, mark, until, w.opts.PageSize)
		if err == nil {
			err = w.evaluateChanged(ctx, changes)
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if progressed {
				st.FreshFailures = 0
			}
			if st.FreshFailures++; st.FreshFailures < followUpFailureLimit {
				return errors.Join(err, w.saveDetached(ctx))
			}
			log.Warn().Err(err).Str("org_id", w.orgID.String()).Msg("inbox tagging: changed threads failed repeatedly; left to the cycle")
			w.p.Skipped++
			if changes == nil {
				changes = []repository.FollowUpChange{}
			}
		}
		if len(changes) < w.opts.PageSize {
			mark = repository.FollowUpMark{At: until, RowID: maxRowID}
		} else {
			last := changes[len(changes)-1]
			mark = repository.FollowUpMark{At: last.At, RowID: last.RowID}
		}
		st.Fresh = &mark
		st.FreshFailures = 0
		progressed = true
		if err := w.save(ctx); err != nil {
			return err
		}
		if len(changes) < w.opts.PageSize {
			return nil
		}
	}
}

func (w *followUpWalk) evaluateChanged(ctx context.Context, changes []repository.FollowUpChange) error {
	var threads []string
	picked := map[string]bool{}
	for _, c := range changes {
		if !w.seen[c.ThreadID] && !picked[c.ThreadID] {
			picked[c.ThreadID] = true
			threads = append(threads, c.ThreadID)
		}
	}
	states, err := w.s.repo.FollowUpThreadStates(ctx, w.orgID, threads, w.opts.Since)
	if err != nil {
		return err
	}
	for _, st := range states {
		if err := w.evaluate(ctx, st); err != nil {
			return err
		}
	}
	return nil
}

// walk evaluates threads from `from` to the end of the last mailbox and reports whether it got there.
func (w *followUpWalk) walk(ctx context.Context, mailboxes []uuid.UUID, from *repository.FollowUpPosition) (bool, error) {
	start := 0
	var after *repository.FollowUpPosition
	if from != nil {
		for start < len(mailboxes) && bytes.Compare(mailboxes[start][:], from.MailboxID[:]) < 0 {
			start++
		}
		if start < len(mailboxes) && mailboxes[start] == from.MailboxID {
			after = from
		}
	}

	first := true
	for _, mailbox := range mailboxes[start:] {
		for {
			// Pages run until the first one that reads anything, so every pass moves the cycle on.
			if !first && w.spent() {
				return false, nil
			}
			page, err := w.s.repo.FollowUpPage(ctx, w.orgID, mailbox, w.opts.Since, after, w.opts.PageSize)
			if err != nil {
				if page, err = w.pageFailed(ctx, mailbox, after, err); err != nil {
					return false, err
				}
			}
			w.p.Pages++
			if page.Rows > 0 {
				first = false
			}

			var last *repository.FollowUpPosition
			for i := range page.States {
				st := page.States[i]
				if err := ctx.Err(); err != nil {
					return false, w.stopAt(ctx, last, err)
				}
				if err := w.evaluate(ctx, st); err != nil {
					if ctx.Err() != nil {
						return false, w.stopAt(ctx, last, ctx.Err())
					}
					if !w.failed() {
						return false, w.stopAt(ctx, last, err)
					}
					log.Warn().Err(err).Str("thread_id", st.ThreadID).Msg("inbox tagging: follow-up label failed repeatedly; thread skipped this cycle")
					w.p.Skipped++
				}
				w.progressed = true
				last = &st.Position
			}
			if page.Last != nil {
				after = page.Last
				if w.state != nil {
					w.state.Cursor = after
					w.state.PageFailures = 0
					w.progressed = true
					if err := w.save(ctx); err != nil {
						return false, err
					}
				}
			}
			if page.Rows < w.opts.PageSize {
				break
			}
		}
		after = nil
	}
	return true, nil
}

func (w *followUpWalk) spent() bool {
	return w.opts.Budget > 0 && time.Since(w.started) >= w.opts.Budget
}

// failed counts a failure at the walk's current place and reports whether it is time to step past it.
func (w *followUpWalk) failed() bool {
	if w.state == nil {
		return false
	}
	if w.progressed {
		w.state.PageFailures = 0
		w.progressed = false
	}
	w.state.PageFailures++
	if w.state.PageFailures < followUpFailureLimit {
		return false
	}
	w.state.PageFailures = 0
	return true
}

// pageFailed steps past a page that keeps failing by reading only where it ends.
func (w *followUpWalk) pageFailed(ctx context.Context, mailbox uuid.UUID, after *repository.FollowUpPosition, cause error) (repository.FollowUpPage, error) {
	if ctx.Err() != nil {
		return repository.FollowUpPage{}, cause
	}
	if !w.failed() {
		return repository.FollowUpPage{}, w.stopAt(ctx, nil, cause)
	}
	page, err := w.s.repo.FollowUpPagePositions(ctx, w.orgID, mailbox, w.opts.Since, after, w.opts.PageSize)
	if err != nil {
		return repository.FollowUpPage{}, err
	}
	log.Warn().Err(cause).Str("org_id", w.orgID.String()).Str("mailbox_id", mailbox.String()).Msg("inbox tagging: follow-up page failed repeatedly; skipped this cycle")
	w.p.Skipped++
	return page, nil
}

// stopAt keeps the cursor on the last thread actually evaluated and returns the cause.
func (w *followUpWalk) stopAt(ctx context.Context, last *repository.FollowUpPosition, cause error) error {
	if w.state == nil {
		return cause
	}
	if last != nil {
		w.state.Cursor = last
	}
	return errors.Join(cause, w.saveDetached(ctx))
}

func (w *followUpWalk) save(ctx context.Context) error {
	if w.state == nil {
		return nil
	}
	ok, err := w.s.repo.SaveFollowUpSweep(ctx, w.orgID, w.owner, w.lease, *w.state)
	if err != nil {
		return err
	}
	if !ok {
		return errLeaseLost
	}
	return nil
}

// saveDetached saves even when the pass's context is already cancelled.
func (w *followUpWalk) saveDetached(ctx context.Context) error {
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return w.save(c)
}

func (w *followUpWalk) release(ctx context.Context) {
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := w.s.repo.ReleaseFollowUpSweep(c, w.orgID, w.owner); err != nil {
		log.Warn().Err(err).Str("org_id", w.orgID.String()).Msg("inbox tagging: follow-up sweep lease not released")
	}
}

func (w *followUpWalk) evaluate(ctx context.Context, st repository.ThreadFollowUpState) error {
	if w.seen[st.ThreadID] {
		return nil
	}
	want := FollowUp(ThreadState{
		ThreadID:       st.ThreadID,
		LastInboundAt:  st.LastInboundAt,
		LastOutboundAt: st.LastOutboundAt,
		BestIntent:     st.BestIntent,
		LastKind:       st.LastKind,
	}, w.now)

	if err := w.s.categories.SyncExclusiveLabels(ctx, w.orgID, st.ThreadID, FollowUpLabels, want); err != nil {
		return err
	}
	w.seen[st.ThreadID] = true
	w.p.Threads++
	if want == "" {
		w.p.Cleared++
	} else {
		w.p.Labelled[want]++
	}
	return nil
}
