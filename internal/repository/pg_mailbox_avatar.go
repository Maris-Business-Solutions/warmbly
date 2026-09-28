package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/warmbly/warmbly/internal/infrastructure/db"
)

// MailboxAvatarCandidate is a mailbox whose profile photo can be read from an
// administrator grant or an inbox vendor, the two sources the control plane holds.
type MailboxAvatarCandidate struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	Email              string
	AvatarURL          string
	DomainGrantID      *uuid.UUID
	VendorConnectionID *uuid.UUID
	VendorMailboxID    string
}

// MailboxAvatarRepository stores the profile photo a mailbox's provider or vendor has set.
type MailboxAvatarRepository interface {
	// Due lists mailboxes with a readable source whose photo was not checked since before.
	Due(ctx context.Context, before time.Time, limit int) ([]MailboxAvatarCandidate, error)
	// Current is the stored photo URL, empty when none or when the mailbox is not in the organization.
	Current(ctx context.Context, orgID, id uuid.UUID) (string, error)
	// Set records the photo URL (empty to clear) and the time it was checked.
	Set(ctx context.Context, orgID, id uuid.UUID, url string) error
	// Checked records a check that found nothing new, keeping the stored photo.
	Checked(ctx context.Context, orgID, id uuid.UUID) error
}

type mailboxAvatarRepository struct{ DB *db.DB }

func NewMailboxAvatarRepository(database *db.DB) MailboxAvatarRepository {
	return &mailboxAvatarRepository{DB: database}
}

func (r *mailboxAvatarRepository) Due(ctx context.Context, before time.Time, limit int) ([]MailboxAvatarCandidate, error) {
	const query = `
		SELECT id, organization_id, email, avatar_url, domain_grant_id, vendor_connection_id, vendor_mailbox_id
		FROM email_accounts
		WHERE status = 'active'
		  AND (domain_grant_id IS NOT NULL OR vendor_connection_id IS NOT NULL)
		  AND (avatar_checked_at IS NULL OR avatar_checked_at < $1)
		ORDER BY avatar_checked_at NULLS FIRST, id
		LIMIT $2`
	rows, err := r.DB.Query(ctx, query, before, limit)
	if err != nil {
		db.CaptureError(err, query, nil, "query")
		return nil, err
	}
	defer rows.Close()
	var out []MailboxAvatarCandidate
	for rows.Next() {
		var c MailboxAvatarCandidate
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Email, &c.AvatarURL, &c.DomainGrantID, &c.VendorConnectionID, &c.VendorMailboxID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *mailboxAvatarRepository) Current(ctx context.Context, orgID, id uuid.UUID) (string, error) {
	const query = `SELECT avatar_url FROM email_accounts WHERE id = $1 AND organization_id = $2`
	var url string
	err := r.DB.QueryRow(ctx, query, id, orgID).Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		db.CaptureError(err, query, nil, "queryrow")
	}
	return url, err
}

func (r *mailboxAvatarRepository) Set(ctx context.Context, orgID, id uuid.UUID, url string) error {
	const query = `UPDATE email_accounts SET avatar_url = $3, avatar_checked_at = now() WHERE id = $1 AND organization_id = $2`
	if _, err := r.DB.Exec(ctx, query, id, orgID, url); err != nil {
		db.CaptureError(err, query, nil, "exec")
		return err
	}
	return nil
}

func (r *mailboxAvatarRepository) Checked(ctx context.Context, orgID, id uuid.UUID) error {
	const query = `UPDATE email_accounts SET avatar_checked_at = now() WHERE id = $1 AND organization_id = $2`
	if _, err := r.DB.Exec(ctx, query, id, orgID); err != nil {
		db.CaptureError(err, query, nil, "exec")
		return err
	}
	return nil
}
