package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

func (r *cloudLinkRepository) ListForOrg(ctx context.Context, orgID uuid.UUID, accountID *uuid.UUID) ([]models.CloudLinkMailbox, error) {
	query := `SELECT ` + cloudLinkMailboxColumns + ` FROM cloud_link_mailboxes
		WHERE email_account_id IN (
			SELECT id FROM email_accounts WHERE organization_id = $1 AND ($2::uuid IS NULL OR id = $2)
		) ORDER BY enrolled_at`
	rows, err := r.db.Query(ctx, query, orgID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.CloudLinkMailbox, 0)
	for rows.Next() {
		m, err := scanCloudLinkMailbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}
