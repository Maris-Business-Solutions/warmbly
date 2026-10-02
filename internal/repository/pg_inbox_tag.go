package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InboxTagResult is one classified inbound message.
type InboxTagResult struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	EmailAccountID   uuid.UUID
	MessageID        string
	ThreadID         string
	Kind             string
	KindConfidence   float64
	KindSource       string
	Intent           string
	IntentConfidence float64
	Relevance        int
	Priority         string
	NeedsReview      bool
	ReviewReason     string
	// Automated is a trusted verdict that no person wrote the message. It is
	// mirrored onto unibox_emails.automated, which is what keeps the
	// conversation out of the inbox.
	Automated bool
	// Campaign is the campaign the verdict was made with, "" when none was
	// known.
	Campaign string
	// ReturnDate is the out-of-office return date the model was asked to
	// confirm, nil when it was not asked. The answer is Answers["return_date"].
	ReturnDate  *time.Time
	Answers     json.RawMessage
	Labels      []string
	Model       string
	InputTokens int
	// Actions is what the workspace's switches let this verdict do: "hold",
	// "stop", "task", "suppress". Empty for a verdict that only labelled.
	Actions   []string
	CreatedAt time.Time
}

type InboxTagRepository interface {
	Claim(ctx context.Context, orgID, accountID uuid.UUID, messageID, threadID string) (bool, error)
	ReleaseClaim(ctx context.Context, orgID uuid.UUID, messageID string) error
	Save(ctx context.Context, r *InboxTagResult) error
	// ListForReview backs the phase-1 review page: what was decided, how
	// confident it was, and what it would have done.
	ListForReview(ctx context.Context, orgID uuid.UUID, limit, offset int, needsReviewOnly bool) ([]InboxTagResult, int, error)
	ReviewSummary(ctx context.Context, orgID uuid.UUID) (InboxTagReviewSummary, error)

	// ListUntagged and PreviousOutbound back the historical backfill.
	ListUntagged(ctx context.Context, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error)
	PreviousOutbound(ctx context.Context, accountID uuid.UUID, threadID string, inReplyTo []string, before time.Time) (string, string, error)
	// ListColdInboundInCampaignThreads and Reopen back the re-check of
	// verdicts made before the campaign behind a thread could be resolved.
	ListColdInboundInCampaignThreads(ctx context.Context, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error)
	Reopen(ctx context.Context, orgID uuid.UUID, messageID, kind string) ([]string, error)
	// ListUncheckedNotifications backs the re-check of notifications stored
	// before they were asked whether they need acting on.
	ListUncheckedNotifications(ctx context.Context, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error)

	// FollowUpMailboxes, FollowUpPage and the cursor pair back the follow-up
	// sweep, which walks each mailbox newest first a page at a time.
	FollowUpMailboxes(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
	FollowUpPage(ctx context.Context, orgID, mailboxID uuid.UUID, since time.Time, after *FollowUpPosition, limit int) (FollowUpPage, error)
	FollowUpCursor(ctx context.Context, orgID uuid.UUID) (*FollowUpPosition, error)
	// SaveFollowUpCursor stores where the sweep stopped; nil ends the cycle.
	SaveFollowUpCursor(ctx context.Context, orgID uuid.UUID, pos *FollowUpPosition) error

	// GetByMessageID reads one completed verdict, so the reply classifier, the
	// inbox agent and the action executor can reuse a judgment already paid
	// for. Nil when the message was never classified.
	GetByMessageID(ctx context.Context, orgID uuid.UUID, messageID string) (*InboxTagResult, error)
	// RecordActions stores what a verdict was allowed to do.
	RecordActions(ctx context.Context, orgID uuid.UUID, messageID string, actions []string) error
}

type inboxTagRepository struct {
	db *pgxpool.Pool
}

func NewInboxTagRepository(db *pgxpool.Pool) InboxTagRepository {
	return &inboxTagRepository{db: db}
}

func (r *inboxTagRepository) Claim(ctx context.Context, orgID, accountID uuid.UUID, messageID, threadID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}
	const q = `
		INSERT INTO inbox_tag_results (
			organization_id, email_account_id, message_id, thread_id, status, claimed_at
		) VALUES ($1, $2, $3, $4, 'processing', NOW())
		ON CONFLICT (organization_id, message_id) DO UPDATE
		SET email_account_id = EXCLUDED.email_account_id,
		    thread_id = EXCLUDED.thread_id,
		    claimed_at = NOW(),
		    updated_at = NOW()
		WHERE inbox_tag_results.status = 'processing'
		  AND inbox_tag_results.claimed_at < NOW() - INTERVAL '15 minutes'
		RETURNING id
	`
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, q, orgID, accountID, messageID, threadID).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *inboxTagRepository) ReleaseClaim(ctx context.Context, orgID uuid.UUID, messageID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM inbox_tag_results
		WHERE organization_id = $1 AND message_id = $2 AND status = 'processing'
	`, orgID, messageID)
	return err
}

func (r *inboxTagRepository) Save(ctx context.Context, res *InboxTagResult) error {
	// One statement, so the verdict and the inbox placement cannot disagree.
	const q = `
		WITH saved AS (
			INSERT INTO inbox_tag_results (
				organization_id, email_account_id, message_id, thread_id,
				kind, kind_confidence, kind_source, intent, intent_confidence,
				relevance, priority, needs_review, review_reason, answers, labels, model, input_tokens,
				automated, campaign, return_date
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
			ON CONFLICT (organization_id, message_id) DO UPDATE SET
				email_account_id = EXCLUDED.email_account_id,
				thread_id = EXCLUDED.thread_id,
				kind = EXCLUDED.kind,
				kind_confidence = EXCLUDED.kind_confidence,
				kind_source = EXCLUDED.kind_source,
				intent = EXCLUDED.intent,
				intent_confidence = EXCLUDED.intent_confidence,
				relevance = EXCLUDED.relevance,
				priority = EXCLUDED.priority,
				needs_review = EXCLUDED.needs_review,
				review_reason = EXCLUDED.review_reason,
				answers = EXCLUDED.answers,
				labels = EXCLUDED.labels,
				model = EXCLUDED.model,
				input_tokens = EXCLUDED.input_tokens,
				automated = EXCLUDED.automated,
				campaign = EXCLUDED.campaign,
				return_date = EXCLUDED.return_date,
				status = 'complete',
				updated_at = NOW()
			RETURNING email_account_id, message_id, automated
		)
		UPDATE unibox_emails ue
		SET automated = saved.automated
		FROM saved
		WHERE ue.email_id = saved.email_account_id
		  AND ue.message_id = saved.message_id
		  AND ue.automated IS DISTINCT FROM saved.automated
	`
	answers := res.Answers
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	labels := res.Labels
	if labels == nil {
		labels = []string{}
	}
	_, err := r.db.Exec(ctx, q,
		res.OrganizationID, res.EmailAccountID, res.MessageID, res.ThreadID,
		res.Kind, res.KindConfidence, res.KindSource, res.Intent, res.IntentConfidence,
		res.Relevance, res.Priority, res.NeedsReview, res.ReviewReason, answers, labels, res.Model, res.InputTokens,
		res.Automated, res.Campaign, res.ReturnDate,
	)
	return err
}

func (r *inboxTagRepository) ListForReview(ctx context.Context, orgID uuid.UUID, limit, offset int, needsReviewOnly bool) ([]InboxTagResult, int, error) {
	where := `WHERE organization_id = $1 AND status = 'complete'`
	if needsReviewOnly {
		where += ` AND needs_review`
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM inbox_tag_results `+where, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+inboxTagColumns+`
		FROM inbox_tag_results `+where+`
		ORDER BY relevance DESC, created_at DESC
		LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]InboxTagResult, 0, limit)
	for rows.Next() {
		x, err := scanInboxTag(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

const inboxTagColumns = `id, organization_id, email_account_id, message_id, thread_id,
		       kind, kind_confidence, kind_source, intent, intent_confidence,
		       relevance, priority, needs_review, review_reason, answers, labels, model, input_tokens, actions, return_date, created_at`

func scanInboxTag(row pgx.Row) (InboxTagResult, error) {
	var x InboxTagResult
	err := row.Scan(
		&x.ID, &x.OrganizationID, &x.EmailAccountID, &x.MessageID, &x.ThreadID,
		&x.Kind, &x.KindConfidence, &x.KindSource, &x.Intent, &x.IntentConfidence,
		&x.Relevance, &x.Priority, &x.NeedsReview, &x.ReviewReason, &x.Answers, &x.Labels, &x.Model, &x.InputTokens, &x.Actions, &x.ReturnDate, &x.CreatedAt,
	)
	return x, err
}

func (r *inboxTagRepository) GetByMessageID(ctx context.Context, orgID uuid.UUID, messageID string) (*InboxTagResult, error) {
	if messageID == "" {
		return nil, nil
	}
	x, err := scanInboxTag(r.db.QueryRow(ctx, `
		SELECT `+inboxTagColumns+`
		FROM inbox_tag_results
		WHERE organization_id = $1 AND message_id = $2 AND status = 'complete'`, orgID, messageID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &x, nil
}

func (r *inboxTagRepository) RecordActions(ctx context.Context, orgID uuid.UUID, messageID string, actions []string) error {
	if actions == nil {
		actions = []string{}
	}
	_, err := r.db.Exec(ctx, `
		UPDATE inbox_tag_results SET actions = $3, updated_at = NOW()
		WHERE organization_id = $1 AND message_id = $2`, orgID, messageID, actions)
	return err
}

type InboxTagReviewSummary struct {
	Total       int
	NeedsReview int
	FromOffline int
	// Acted counts verdicts that held, stopped, opened a task or suppressed.
	Acted int
}

func (r *inboxTagRepository) ReviewSummary(ctx context.Context, orgID uuid.UUID) (InboxTagReviewSummary, error) {
	const q = `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE needs_review),
		       COUNT(*) FILTER (WHERE kind_source = 'header'),
		       COUNT(*) FILTER (WHERE cardinality(actions) > 0)
		FROM inbox_tag_results
		WHERE organization_id = $1 AND status = 'complete'
	`
	var out InboxTagReviewSummary
	err := r.db.QueryRow(ctx, q, orgID).Scan(&out.Total, &out.NeedsReview, &out.FromOffline, &out.Acted)
	return out, err
}

// BackfillCandidate is one historical message the backfill may classify.
type BackfillCandidate struct {
	EmailAccountID uuid.UUID
	UserID         uuid.UUID
	MessageID      string
	ThreadID       string
	Subject        string
	BodyText       string
	FromAddr       string
	InReplyTo      []string
	// Flags carries the classification headers the sync stores as pseudo-flags.
	Flags        []string
	InternalDate time.Time
}

// ListUntagged returns inbound messages that have never been classified, newest
// first, for the backfill.
//
// Three exclusions, all deliberate:
//
//   - folder = 'inbox' only. Our own sends are never classified, and the folder
//     is the fact that says which is which. Reading direction from content is
//     how our own outbound gets labelled a human reply at 0.94 confidence.
//   - a sender that is one of our own mailboxes is dropped even inside the
//     inbox folder: mail between two connected mailboxes lands in the second
//     one's inbox and is still ours.
//   - anything already in inbox_tag_results, so a re-run resumes rather than
//     repeats. Same key the live path is idempotent on.
func (r *inboxTagRepository) ListUntagged(ctx context.Context, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error) {
	return r.listCandidates(ctx, `
		  AND NOT EXISTS (
		        SELECT 1 FROM inbox_tag_results r
		        WHERE r.organization_id = $1 AND r.message_id = ue.message_id
		          AND (r.status = 'complete' OR r.claimed_at >= NOW() - INTERVAL '15 minutes')
		      )`, orgID, since, limit)
}

// ListColdInboundInCampaignThreads returns inbound messages stored as
// cold_inbound, by a verdict made without a campaign, whose thread a campaign
// send of the same mailbox answers for: by the Gmail thread handle, by a
// Message-ID the reply names, or by the sent copy in the thread. A verdict
// made with the campaign in front of it is not asked again.
func (r *inboxTagRepository) ListColdInboundInCampaignThreads(ctx context.Context, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error) {
	return r.listCandidates(ctx, `
		  AND EXISTS (
		        SELECT 1 FROM inbox_tag_results r
		        WHERE r.organization_id = $1 AND r.message_id = ue.message_id
		          AND r.status = 'complete' AND r.kind = 'cold_inbound' AND r.campaign = ''
		      )
		  AND EXISTS (
		        SELECT 1
		        FROM tasks t
		        JOIN campaign_tasks ct ON ct.task_id = t.id
		        JOIN campaigns c ON c.id = ct.campaign_id
		        WHERE t.email_account_id = ue.email_id AND t.task_type = 'campaign'
		          AND (
		                (ue.thread_id <> '' AND t.thread_id = ue.thread_id)
		             OR (t.message_id <> '' AND t.message_id IN (
		                    SELECT v FROM (
		                        SELECT BTRIM(ref, '<> ') AS id FROM unnest(COALESCE(ue.in_reply_to, '{}')) AS ref
		                        UNION
		                        SELECT BTRIM(s.message_id, '<> ') FROM unibox_emails s
		                        WHERE s.email_id = ue.email_id AND s.thread_id = ue.thread_id
		                          AND ue.thread_id <> '' AND s.folder = 'sent'
		                    ) ids, LATERAL (VALUES (ids.id), ('<' || ids.id || '>')) AS forms(v)
		                    WHERE ids.id <> ''
		                ))
		          )
		      )`, orgID, since, limit)
}

// ListUncheckedNotifications returns inbound messages stored as notifications
// by a verdict that never asked whether they need the recipient to act.
func (r *inboxTagRepository) ListUncheckedNotifications(ctx context.Context, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error) {
	return r.listCandidates(ctx, `
		  AND EXISTS (
		        SELECT 1 FROM inbox_tag_results r
		        WHERE r.organization_id = $1 AND r.message_id = ue.message_id
		          AND r.status = 'complete' AND r.kind = 'notification'
		          AND NOT (r.answers ? 'action_required')
		      )`, orgID, since, limit)
}

func (r *inboxTagRepository) listCandidates(ctx context.Context, filter string, orgID uuid.UUID, since time.Time, limit int) ([]BackfillCandidate, error) {
	q := `
		SELECT ue.email_id, ue.user_id, ue.message_id, ue.thread_id,
		       ue.subject, ue.body_text, COALESCE(ue.from_addr[1], ''), COALESCE(ue.in_reply_to, '{}'), ue.flags, ue.internal_date
		FROM unibox_emails ue
		JOIN email_accounts ea ON ea.id = ue.email_id
		WHERE ea.organization_id = $1
		  AND ue.folder = 'inbox'
		  AND ue.internal_date >= $2
		  AND ue.message_id <> ''
		  AND LOWER(COALESCE(
		        NULLIF((regexp_match(COALESCE(ue.from_addr[1], ''), '<([^<>]+)>\s*$'))[1], ''),
		        NULLIF((regexp_match(COALESCE(ue.from_addr[1], ''), '\(([^()]+)\)\s*$'))[1], ''),
		        TRIM(COALESCE(ue.from_addr[1], ''))
		      ))
		      NOT IN (SELECT LOWER(email) FROM email_accounts WHERE organization_id = $1)` + filter + `
		ORDER BY ue.internal_date DESC
		LIMIT $3
	`
	rows, err := r.db.Query(ctx, q, orgID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BackfillCandidate
	for rows.Next() {
		var c BackfillCandidate
		if err := rows.Scan(&c.EmailAccountID, &c.UserID, &c.MessageID, &c.ThreadID,
			&c.Subject, &c.BodyText, &c.FromAddr, &c.InReplyTo, &c.Flags, &c.InternalDate); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Reopen drops one stored verdict of the given kind so the message can be
// classified again, and returns the labels it had written. Only a complete
// verdict of that kind is touched.
// The message returns to the inbox with it, so a failed re-classification leaves it visible.
func (r *inboxTagRepository) Reopen(ctx context.Context, orgID uuid.UUID, messageID, kind string) ([]string, error) {
	var labels []string
	err := r.db.QueryRow(ctx, `
		WITH dropped AS (
			DELETE FROM inbox_tag_results
			WHERE organization_id = $1 AND message_id = $2 AND status = 'complete' AND kind = $3
			RETURNING email_account_id, message_id, labels
		), shown AS (
			UPDATE unibox_emails ue
			SET automated = false
			FROM dropped
			WHERE ue.email_id = dropped.email_account_id
			  AND ue.message_id = dropped.message_id
			  AND ue.automated
		)
		SELECT labels FROM dropped
	`, orgID, messageID, kind).Scan(&labels)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return labels, err
}

// PreviousOutbound is the plain text of the last message we sent in a thread
// before a given moment, and the name of the campaign the thread belongs to.
//
// Without it a reply cannot be read: "yes", "that works" and "sounds good" are
// answers, and the question they answer is not in them. Giving the model our
// side of the exchange is what lets the reply mean anything.
//
// The campaign resolves from any of three facts, all scoped to the mailbox: the
// sent copy's Message-ID, a Message-ID the reply names in In-Reply-To, or the
// provider thread handle the worker recorded on the send (Gmail only). The
// handle is what holds when the Message-ID on the task is not the one the
// provider put on the wire.
func (r *inboxTagRepository) PreviousOutbound(ctx context.Context, accountID uuid.UUID, threadID string, inReplyTo []string, before time.Time) (string, string, error) {
	if threadID == "" && len(inReplyTo) == 0 {
		return "", "", nil
	}
	if inReplyTo == nil {
		inReplyTo = []string{}
	}
	const q = `
		WITH prev AS (
			SELECT ue.body_text, ue.message_id
			FROM unibox_emails ue
			WHERE $2 <> '' AND ue.email_id = $1 AND ue.thread_id = $2
			  AND ue.folder = 'sent' AND ue.internal_date < $3
			ORDER BY ue.internal_date DESC
			LIMIT 1
		),
		ids AS (
			SELECT DISTINCT x FROM (
				SELECT BTRIM(message_id, '<> ') AS x FROM prev
				UNION ALL
				SELECT BTRIM(ref, '<> ') FROM unnest($4::text[]) AS ref
			) raw
			WHERE x <> ''
		),
		matched AS (
			SELECT t.id, 0 AS rank, t.created_at
			FROM tasks t
			WHERE t.message_id <> ''
			  AND t.message_id IN (SELECT x FROM ids UNION ALL SELECT '<' || x || '>' FROM ids)
			  AND t.email_account_id = $1 AND t.task_type = 'campaign'
			UNION ALL
			SELECT t.id, 1 AS rank, t.created_at
			FROM tasks t
			WHERE $2 <> '' AND t.email_account_id = $1 AND t.thread_id = $2
			  AND t.task_type = 'campaign'
		)
		SELECT COALESCE((SELECT body_text FROM prev), ''),
		       COALESCE((
		           SELECT c.name
		           FROM matched m
		           JOIN campaign_tasks ct ON ct.task_id = m.id
		           JOIN campaigns c ON c.id = ct.campaign_id
		           ORDER BY m.rank, m.created_at DESC
		           LIMIT 1
		       ), '')
	`
	var body, campaign string
	if err := r.db.QueryRow(ctx, q, accountID, threadID, before, inReplyTo).Scan(&body, &campaign); err != nil {
		return "", "", err
	}
	return body, campaign, nil
}
