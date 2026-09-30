package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/config"
)

// Verdicts a warmup spam move is attributed to.
const (
	SpamMovePending      = "pending"
	SpamMoveOwner        = "owner"
	SpamMoveProvider     = "provider"
	SpamMoveUnattributed = "unattributed"
)

// WarmupSpamMove is a received warmup email seen moving into spam after it
// arrived, which no provider attributes to anyone.
type WarmupSpamMove struct {
	EmailAccountID  uuid.UUID
	MessageID       string
	SenderAccountID uuid.UUID
	ReceivedAt      time.Time
	ObservedAt      time.Time
}

// WarmupSpamMoveEvidence is what the pool and the mailbox show around one move.
type WarmupSpamMoveEvidence struct {
	// OwnerActiveNear: the owner acted on their own mail within
	// config.WarmupSpamMoveActivityMinutes of the move.
	OwnerActiveNear bool
	// OwnerActiveRecently: any owner activity in config.WarmupOwnerDormantDays.
	OwnerActiveRecently bool
	// CorrelatedElsewhere counts the same sender's mail moved to spam in other
	// workspaces within config.WarmupSpamMoveCorrelationHours.
	CorrelatedElsewhere int
	// PatternSenders is distinct senders across this mailbox's unexplained
	// moves in the last seven days, this one included.
	PatternSenders int
}

// RecordWarmupSpamMove holds a move for attribution; the first sighting wins.
func (r *warmupRepository) RecordWarmupSpamMove(ctx context.Context, m WarmupSpamMove) (bool, error) {
	cmd, err := r.db.Exec(ctx, `
		INSERT INTO warmup_spam_moves (email_account_id, message_id, sender_account_id, received_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email_account_id, message_id) DO NOTHING`,
		m.EmailAccountID, m.MessageID, m.SenderAccountID, m.ReceivedAt)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

// ListSettledWarmupSpamMoves is pending moves observed before settledBefore, oldest first.
func (r *warmupRepository) ListSettledWarmupSpamMoves(ctx context.Context, settledBefore time.Time, limit int) ([]WarmupSpamMove, error) {
	rows, err := r.db.Query(ctx, `
		SELECT email_account_id, message_id, sender_account_id, received_at, observed_at
		FROM warmup_spam_moves
		WHERE verdict = 'pending' AND observed_at < $1
		ORDER BY observed_at
		LIMIT $2`, settledBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WarmupSpamMove
	for rows.Next() {
		var m WarmupSpamMove
		if err := rows.Scan(&m.EmailAccountID, &m.MessageID, &m.SenderAccountID, &m.ReceivedAt, &m.ObservedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// WarmupSpamMoveEvidence gathers the evidence for one move in one round trip.
func (r *warmupRepository) WarmupSpamMoveEvidence(ctx context.Context, m WarmupSpamMove) (WarmupSpamMoveEvidence, error) {
	var ev WarmupSpamMoveEvidence
	err := r.db.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM mailbox_owner_activity a
			        WHERE a.email_account_id = $1
			          AND a.bucket >= $2::timestamptz - make_interval(mins => $4 + 5)
			          AND a.bucket <= $2::timestamptz + make_interval(mins => $4)),
			EXISTS (SELECT 1 FROM mailbox_owner_activity a
			        WHERE a.email_account_id = $1
			          AND a.bucket >= $2::timestamptz - make_interval(days => $5)),
			(SELECT COUNT(*) FROM warmup_spam_moves o
			   JOIN email_accounts oa ON oa.id = o.email_account_id
			  WHERE o.sender_account_id = $3
			    AND o.email_account_id <> $1
			    AND oa.organization_id IS DISTINCT FROM (SELECT organization_id FROM email_accounts WHERE id = $1)
			    AND o.observed_at BETWEEN $2::timestamptz - make_interval(hours => $6) AND $2::timestamptz + make_interval(hours => $6)),
			(SELECT COUNT(DISTINCT s) FROM (
			    SELECT sender_account_id AS s FROM warmup_spam_moves
			     WHERE email_account_id = $1
			       AND verdict IN ('owner', 'unattributed')
			       AND observed_at >= $2::timestamptz - INTERVAL '7 days' AND observed_at <= $2::timestamptz
			    UNION SELECT $3::uuid) p)`,
		m.EmailAccountID, m.ObservedAt, m.SenderAccountID,
		config.WarmupSpamMoveActivityMinutes, config.WarmupOwnerDormantDays, config.WarmupSpamMoveCorrelationHours,
	).Scan(&ev.OwnerActiveNear, &ev.OwnerActiveRecently, &ev.CorrelatedElsewhere, &ev.PatternSenders)
	return ev, err
}

// DecideWarmupSpamMove settles a pending move once; false when it was already decided.
func (r *warmupRepository) DecideWarmupSpamMove(ctx context.Context, accountID uuid.UUID, messageID, verdict string, signals []string) (bool, error) {
	cmd, err := r.db.Exec(ctx, `
		UPDATE warmup_spam_moves
		SET verdict = $3, signals = $4, decided_at = NOW()
		WHERE email_account_id = $1 AND message_id = $2 AND verdict = 'pending'`,
		accountID, messageID, verdict, signals)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

// correlatedOwnerMovesWhere selects the sender's owner-attributed moves in
// other workspaces near $3, for sender $1 and the correlating mailbox $2.
const correlatedOwnerMovesWhere = `
	o.sender_account_id = $1
	AND o.email_account_id <> $2
	AND o.verdict = 'owner'
	AND o.email_account_id IN (
		SELECT oa.id FROM email_accounts oa
		WHERE oa.organization_id IS DISTINCT FROM (SELECT organization_id FROM email_accounts WHERE id = $2))
	AND o.observed_at BETWEEN $3::timestamptz - make_interval(hours => $4) AND $3::timestamptz + make_interval(hours => $4)`

// CorrelatedOwnerSpamMoves lists the owner verdicts a correlation now explains.
func (r *warmupRepository) CorrelatedOwnerSpamMoves(ctx context.Context, senderID, exceptAccountID uuid.UUID, at time.Time) ([]WarmupSpamMove, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.email_account_id, o.message_id, o.sender_account_id, o.received_at, o.observed_at
		FROM warmup_spam_moves o
		WHERE `+correlatedOwnerMovesWhere,
		senderID, exceptAccountID, at, config.WarmupSpamMoveCorrelationHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WarmupSpamMove
	for rows.Next() {
		var m WarmupSpamMove
		if err := rows.Scan(&m.EmailAccountID, &m.MessageID, &m.SenderAccountID, &m.ReceivedAt, &m.ObservedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ReattributeOwnerSpamMoves turns those moves into provider moves, and the
// complaint each filed against the sender into placement.
func (r *warmupRepository) ReattributeOwnerSpamMoves(ctx context.Context, senderID, exceptAccountID uuid.UUID, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		WITH moved AS (
			UPDATE warmup_spam_moves o
			SET verdict = 'provider', signals = array_append(o.signals, 'correlated_later'), decided_at = NOW()
			WHERE `+correlatedOwnerMovesWhere+`
			RETURNING o.email_account_id, o.message_id
		)
		UPDATE warmup_spam_reports sr
		SET report_type = 'spam_placement'
		FROM moved
		WHERE sr.reporter_account_id = moved.email_account_id
		  AND sr.message_id = moved.message_id
		  AND sr.report_type = 'user_complaint'`,
		senderID, exceptAccountID, at, config.WarmupSpamMoveCorrelationHours)
	return err
}

// RecordOwnerActivity marks the five-minute bucket the owner acted in.
func (r *warmupRepository) RecordOwnerActivity(ctx context.Context, accountID uuid.UUID, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO mailbox_owner_activity (email_account_id, bucket)
		VALUES ($1, to_timestamp(floor(extract(epoch FROM $2::timestamptz) / 300) * 300))
		ON CONFLICT (email_account_id, bucket) DO UPDATE SET events = mailbox_owner_activity.events + 1`,
		accountID, at)
	return err
}
