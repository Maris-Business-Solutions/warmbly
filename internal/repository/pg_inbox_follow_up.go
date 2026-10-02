package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ThreadFollowUpState is one thread's follow-up facts. Every field is read from
// the database; none of it is inferred, and none of it is asked of a model.
type ThreadFollowUpState struct {
	ThreadID       string
	LastInboundAt  time.Time
	LastOutboundAt time.Time
	// BestIntent is the most recent trusted intent in this thread.
	BestIntent string
	// LastKind is the classified kind of the newest inbound message, which is
	// what says whether the "reply" was a person or a mail server.
	LastKind string
	// Position is the thread's newest message, where the sweep stands once it is evaluated.
	Position FollowUpPosition
}

// FollowUpPosition is one message row in the walk: mailboxes by id, then (internal_date, id) descending.
type FollowUpPosition struct {
	MailboxID uuid.UUID
	At        time.Time
	RowID     uuid.UUID
}

// FollowUpPage is one bounded step of the walk.
type FollowUpPage struct {
	// States are the page's threads that we have written to, in walk order.
	States []ThreadFollowUpState
	// Rows is how many messages the page read; fewer than the limit ends the mailbox.
	Rows int
	// Last is the last message read, nil when the page was empty.
	Last *FollowUpPosition
}

func (r *inboxTagRepository) FollowUpMailboxes(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM email_accounts WHERE organization_id = $1 ORDER BY id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// FollowUpPage reads one bounded page of a mailbox and resolves facts only for threads whose newest workspace message is on it.
func (r *inboxTagRepository) FollowUpPage(ctx context.Context, orgID, mailboxID uuid.UUID, since time.Time, after *FollowUpPosition, limit int) (FollowUpPage, error) {
	args := []any{orgID, mailboxID, since, limit}
	keyset := ""
	if after != nil {
		keyset = `AND (ue.internal_date, ue.id) < ($5, $6)`
		args = append(args, after.At, after.RowID)
	}
	q := `
	WITH page AS MATERIALIZED (
		SELECT ue.id, ue.internal_date, ue.thread_id
		FROM unibox_emails ue
		WHERE ue.email_id = $2
		  AND EXISTS (SELECT 1 FROM email_accounts ea WHERE ea.id = $2 AND ea.organization_id = $1)
		  AND ue.internal_date >= $3 ` + keyset + `
		  AND ue.thread_id <> ''
		ORDER BY ue.internal_date DESC, ue.id DESC
		LIMIT $4
	),
	heads AS (
		SELECT p.id, p.thread_id
		FROM page p
		WHERE NOT EXISTS (
			SELECT 1
			FROM unibox_emails o
			JOIN email_accounts oa ON oa.id = o.email_id AND oa.organization_id = $1
			WHERE o.thread_id = p.thread_id
			  AND (o.internal_date, o.id) > (p.internal_date, p.id)
		)
	),
	states AS (
		SELECT h.id, agg.last_in, agg.last_out,
		       COALESCE(best.intent, '') AS intent, COALESCE(newest.kind, '') AS kind
		FROM heads h
		CROSS JOIN LATERAL (
			SELECT MAX(o.internal_date) FILTER (WHERE o.folder = 'inbox') AS last_in,
			       MAX(o.internal_date) FILTER (WHERE o.folder = 'sent')  AS last_out
			FROM unibox_emails o
			JOIN email_accounts oa ON oa.id = o.email_id AND oa.organization_id = $1
			WHERE o.thread_id = h.thread_id
		) agg
		LEFT JOIN LATERAL (
			SELECT o.email_id, o.message_id
			FROM unibox_emails o
			JOIN email_accounts oa ON oa.id = o.email_id AND oa.organization_id = $1
			WHERE o.thread_id = h.thread_id AND o.folder = 'inbox'
			ORDER BY o.internal_date DESC, o.id DESC
			LIMIT 1
		) latest ON TRUE
		LEFT JOIN LATERAL (
			SELECT r.intent
			FROM inbox_tag_results r
			JOIN unibox_emails ue
			  ON ue.email_id = r.email_account_id
			 AND ue.thread_id = r.thread_id
			 AND ue.message_id = r.message_id
			JOIN email_accounts oa ON oa.id = ue.email_id AND oa.organization_id = $1
			WHERE r.organization_id = $1 AND r.thread_id = h.thread_id
			  AND r.status = 'complete' AND r.review_reason <> 'intent' AND r.intent <> ''
			ORDER BY ue.internal_date DESC, r.created_at DESC
			LIMIT 1
		) best ON TRUE
		LEFT JOIN inbox_tag_results newest
		  ON newest.organization_id = $1
		 AND newest.email_account_id = latest.email_id
		 AND newest.thread_id = h.thread_id
		 AND newest.message_id = latest.message_id
		 AND newest.status = 'complete'
		 AND newest.review_reason <> 'kind'
		WHERE agg.last_out IS NOT NULL
	)
	SELECT p.id, p.internal_date, p.thread_id, s.id IS NOT NULL,
	       s.last_in, s.last_out, COALESCE(s.intent, ''), COALESCE(s.kind, '')
	FROM page p
	LEFT JOIN states s ON s.id = p.id
	ORDER BY p.internal_date DESC, p.id DESC
	`
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return FollowUpPage{}, err
	}
	defer rows.Close()

	var out FollowUpPage
	for rows.Next() {
		var (
			pos             = FollowUpPosition{MailboxID: mailboxID}
			threadID        string
			due             bool
			lastIn, lastOut *time.Time
			intent, kind    string
		)
		if err := rows.Scan(&pos.RowID, &pos.At, &threadID, &due, &lastIn, &lastOut, &intent, &kind); err != nil {
			return FollowUpPage{}, err
		}
		out.Rows++
		last := pos
		out.Last = &last
		if !due {
			continue
		}
		st := ThreadFollowUpState{ThreadID: threadID, BestIntent: intent, LastKind: kind, Position: pos}
		if lastIn != nil {
			st.LastInboundAt = *lastIn
		}
		if lastOut != nil {
			st.LastOutboundAt = *lastOut
		}
		out.States = append(out.States, st)
	}
	return out, rows.Err()
}

func (r *inboxTagRepository) FollowUpCursor(ctx context.Context, orgID uuid.UUID) (*FollowUpPosition, error) {
	var pos FollowUpPosition
	err := r.db.QueryRow(ctx, `
		SELECT email_account_id, internal_date, message_row_id
		FROM inbox_follow_up_sweeps WHERE organization_id = $1`, orgID).Scan(&pos.MailboxID, &pos.At, &pos.RowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pos, nil
}

func (r *inboxTagRepository) SaveFollowUpCursor(ctx context.Context, orgID uuid.UUID, pos *FollowUpPosition) error {
	if pos == nil {
		_, err := r.db.Exec(ctx, `DELETE FROM inbox_follow_up_sweeps WHERE organization_id = $1`, orgID)
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO inbox_follow_up_sweeps (organization_id, email_account_id, internal_date, message_row_id, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (organization_id) DO UPDATE SET
			email_account_id = EXCLUDED.email_account_id,
			internal_date = EXCLUDED.internal_date,
			message_row_id = EXCLUDED.message_row_id,
			updated_at = NOW()`, orgID, pos.MailboxID, pos.At, pos.RowID)
	return err
}
