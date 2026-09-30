package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CustomDomainRepository answers the one question the on-demand TLS gate asks:
// is this hostname a custom domain some workspace on this instance has already
// verified? Read-only by construction; nothing here mints or changes a domain.
type CustomDomainRepository interface {
	IsVerified(ctx context.Context, host string) (bool, error)
}

type customDomainRepository struct {
	db *pgxpool.Pool
}

func NewCustomDomainRepository(db *pgxpool.Pool) CustomDomainRepository {
	return &customDomainRepository{db: db}
}

// IsVerified reports whether host is a verified custom tracking domain (on a
// mailbox or a campaign override), a verified custom forms domain, or a
// verified root redirect this instance serves itself.
//
// Verified is the whole gate. Verification is a DNS check, so it completes
// before any certificate is needed and there is no ordering problem, and a
// domain that has not passed it is never built into a link, which makes it a
// name nothing legitimate will ever ask for.
//
// Each arm is a partial-index hit (migration 000144) and the query stops at
// the first match.
func (r *customDomainRepository) IsVerified(ctx context.Context, host string) (bool, error) {
	if host == "" {
		return false, nil
	}
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM email_accounts
			WHERE tracking_domain = $1 AND tracking_domain_verified
			UNION ALL
			SELECT 1 FROM campaigns
			WHERE tracking_domain = $1 AND tracking_domain_verified
			UNION ALL
			SELECT 1 FROM organizations
			WHERE forms_domain = $1 AND forms_domain_verified
			UNION ALL
			SELECT 1 FROM domain_redirects
			WHERE verified AND served_by = 'instance' AND (domain = $1 OR (include_www AND 'www.' || domain = $1))
		)
	`, host).Scan(&ok)
	if err != nil {
		return false, err
	}
	return ok, nil
}
