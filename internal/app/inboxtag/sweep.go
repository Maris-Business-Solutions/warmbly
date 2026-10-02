package inboxtag

import (
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/repository"
)

// DefaultFollowUpPageSize is how many messages one page of the sweep reads.
const DefaultFollowUpPageSize = 500

// FollowUpSweep bounds one pass of the follow-up sweep.
type FollowUpSweep struct {
	// Since is the oldest activity a thread may have and still be swept.
	Since time.Time
	// Fresh sweeps threads active this recently first, every pass.
	Fresh time.Duration
	// Budget stops a pass from starting another page; zero runs the cycle to its end.
	Budget time.Duration
	// PageSize is how many messages a page reads; zero is DefaultFollowUpPageSize.
	PageSize int
	// Full sweeps every thread from the newest and leaves the scheduled cursor alone.
	Full bool
}

// FollowUpProgress reports what one sweep changed.
type FollowUpProgress struct {
	Threads  int
	Labelled map[string]int
	Cleared  int
	Pages    int
	// Complete is a cycle that reached its end, so the next pass starts at the newest.
	Complete bool
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

	var cursor *repository.FollowUpPosition
	if !opts.Full {
		c, err := s.repo.FollowUpCursor(ctx, orgID)
		if err != nil {
			return p, err
		}
		cursor = c
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

	// New mail is checked every pass, so a reply never waits for the cycle to reach its mailbox.
	if opts.Fresh > 0 {
		fresh := now.Add(-opts.Fresh)
		if fresh.Before(opts.Since) {
			fresh = opts.Since
		}
		if _, err := w.walk(ctx, mailboxes, fresh, nil, nil); err != nil {
			return p, err
		}
	}

	var save func(context.Context, *repository.FollowUpPosition) error
	if !opts.Full {
		save = func(c context.Context, pos *repository.FollowUpPosition) error {
			return s.repo.SaveFollowUpCursor(c, orgID, pos)
		}
	}
	complete, err := w.walk(ctx, mailboxes, opts.Since, cursor, save)
	if err != nil || !complete {
		return p, err
	}
	if save != nil {
		if err := save(ctx, nil); err != nil {
			return p, err
		}
	}
	p.Complete = true
	return p, nil
}

type followUpWalk struct {
	s       *Service
	orgID   uuid.UUID
	opts    FollowUpSweep
	started time.Time
	now     time.Time
	seen    map[string]bool
	p       *FollowUpProgress
}

// walk evaluates threads from `from` to the end of the last mailbox and reports whether it got there.
func (w *followUpWalk) walk(ctx context.Context, mailboxes []uuid.UUID, since time.Time, from *repository.FollowUpPosition, save func(context.Context, *repository.FollowUpPosition) error) (bool, error) {
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
			if !first && w.opts.Budget > 0 && time.Since(w.started) >= w.opts.Budget {
				return false, nil
			}

			page, err := w.s.repo.FollowUpPage(ctx, w.orgID, mailbox, since, after, w.opts.PageSize)
			if err != nil {
				return false, err
			}
			w.p.Pages++
			if page.Rows > 0 {
				first = false
			}

			var last *repository.FollowUpPosition
			for i := range page.States {
				st := page.States[i]
				if err := ctx.Err(); err != nil {
					return false, w.interrupted(ctx, save, last, err)
				}
				if err := w.evaluate(ctx, st); err != nil && ctx.Err() != nil {
					return false, w.interrupted(ctx, save, last, ctx.Err())
				}
				last = &st.Position
			}
			if page.Last != nil {
				after = page.Last
				if save != nil {
					if err := save(ctx, after); err != nil {
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

// interrupted keeps the cursor on the last thread actually evaluated.
func (w *followUpWalk) interrupted(ctx context.Context, save func(context.Context, *repository.FollowUpPosition) error, last *repository.FollowUpPosition, cause error) error {
	if save == nil || last == nil {
		return cause
	}
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := save(c, last); err != nil {
		log.Warn().Err(err).Str("org_id", w.orgID.String()).Msg("inbox tagging: follow-up sweep position not saved")
	}
	return cause
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
		log.Warn().Err(err).Str("thread_id", st.ThreadID).Msg("inbox tagging: follow-up label not applied")
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
