package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/models"
)

// ErrDeveloperBlockExists is returned when the workspace or person is already blocked.
var ErrDeveloperBlockExists = errors.New("developer block exists")

// OAuthAdminRepository is the operator's view of every OAuth app on the
// instance: moderation reads and the suspend, revoke and block actions.
type OAuthAdminRepository interface {
	ListApps(ctx context.Context, s *models.AdminOAuthAppSearch) ([]models.AdminOAuthApp, int64, error)
	GetApp(ctx context.Context, id uuid.UUID) (*models.AdminOAuthApp, error)
	// Suspend marks the app suspended; it returns false when it already was.
	Suspend(ctx context.Context, id, adminID uuid.UUID, reason string) (bool, error)
	Unsuspend(ctx context.Context, id uuid.UUID) error
	// RevokeAllGrants ends every token the app holds and returns how many.
	RevokeAllGrants(ctx context.Context, id uuid.UUID) (int64, error)
	ClearLogo(ctx context.Context, id uuid.UUID) error
	// SuspendOrgApps suspends every unsuspended app the workspace owns and returns their ids.
	SuspendOrgApps(ctx context.Context, orgID, adminID uuid.UUID, reason string) ([]uuid.UUID, error)
	// SuspendCreatorApps suspends every unsuspended app the person registered and returns their ids.
	SuspendCreatorApps(ctx context.Context, userID, adminID uuid.UUID, reason string) ([]uuid.UUID, error)

	ListBlocks(ctx context.Context) ([]models.OAuthDeveloperBlock, error)
	CreateBlock(ctx context.Context, orgID, userID *uuid.UUID, adminID uuid.UUID, reason string) (*models.OAuthDeveloperBlock, error)
	DeleteBlock(ctx context.Context, id uuid.UUID) (bool, error)
}

type oauthAdminRepository struct {
	db *pgxpool.Pool
}

func NewOAuthAdminRepository(db *pgxpool.Pool) OAuthAdminRepository {
	return &oauthAdminRepository{db: db}
}

const adminOAuthAppSelect = `
	SELECT a.id, a.organization_id, COALESCE(o.name, ''), a.created_by, COALESCE(u.email, ''),
		a.name, a.description, a.logo_url, a.website_url, a.client_id, a.redirect_uris, a.webhook_url,
		a.scopes, a.status, a.is_public, a.suspended_at, a.suspended_reason,
		(SELECT count(DISTINCT g.organization_id) FROM oauth_access_grants g
			WHERE g.application_id = a.id AND g.revoked_at IS NULL AND (g.refresh_expires_at IS NULL OR g.refresh_expires_at > now()))::int,
		COALESCE(l.slug, ''), COALESCE(l.status, ''),
		EXISTS (SELECT 1 FROM oauth_developer_blocks b WHERE b.organization_id = a.organization_id),
		EXISTS (SELECT 1 FROM oauth_developer_blocks b WHERE b.user_id = a.created_by),
		a.created_at
	FROM oauth_applications a
	LEFT JOIN organizations o ON o.id = a.organization_id
	LEFT JOIN users u ON u.id = a.created_by
	LEFT JOIN app_directory_listings l ON l.application_id = a.id`

func scanAdminOAuthApp(row pgx.Row) (*models.AdminOAuthApp, error) {
	var a models.AdminOAuthApp
	var orgID, createdBy *uuid.UUID
	var scopes int64
	var status string
	if err := row.Scan(&a.ID, &orgID, &a.OrganizationName, &createdBy, &a.CreatedByEmail,
		&a.Name, &a.Description, &a.LogoURL, &a.WebsiteURL, &a.ClientID, &a.RedirectURIs, &a.WebhookURL,
		&scopes, &status, &a.IsPublic, &a.SuspendedAt, &a.SuspendedReason,
		&a.Installs, &a.ListingSlug, &a.ListingStatus, &a.OrgBlocked, &a.CreatorBlocked, &a.CreatedAt); err != nil {
		return nil, err
	}
	if orgID != nil {
		a.OrganizationID = *orgID
	}
	if createdBy != nil {
		a.CreatedBy = *createdBy
	}
	if a.RedirectURIs == nil {
		a.RedirectURIs = []string{}
	}
	a.Permissions = models.PermissionsIn(uint64(scopes))
	a.Status = models.OAuthAppStatus(status)
	return &a, nil
}

func adminOAuthAppFilter(s *models.AdminOAuthAppSearch, first int) (string, []any) {
	// Dynamically registered clients belong to nobody until a person approves
	// them; they are MCP plumbing, not apps anyone published.
	where := []string{"a.organization_id IS NOT NULL"}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return "$" + itoa(first+len(args)-1)
	}
	switch s.Status {
	case "suspended":
		where = append(where, "a.suspended_at IS NOT NULL")
	case "active":
		where = append(where, "a.suspended_at IS NULL AND a.status = 'active'")
	case "disabled":
		where = append(where, "a.suspended_at IS NULL AND a.status = 'disabled'")
	}
	if q := strings.TrimSpace(s.Q); q != "" {
		n := next("%" + escapeLike(q) + "%")
		where = append(where, "(a.name ILIKE "+n+" OR a.client_id ILIKE "+n+" OR o.name ILIKE "+n+" OR u.email ILIKE "+n+" OR a.website_url ILIKE "+n+")")
	}
	return strings.Join(where, " AND "), args
}

func (r *oauthAdminRepository) ListApps(ctx context.Context, s *models.AdminOAuthAppSearch) ([]models.AdminOAuthApp, int64, error) {
	cond, args := adminOAuthAppFilter(s, 1)
	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM oauth_applications a
		LEFT JOIN organizations o ON o.id = a.organization_id
		LEFT JOIN users u ON u.id = a.created_by
		WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, s.Limit, s.Offset)
	rows, err := r.db.Query(ctx, adminOAuthAppSelect+`
		WHERE `+cond+`
		ORDER BY (a.suspended_at IS NOT NULL) DESC, a.created_at DESC, a.id
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.AdminOAuthApp{}
	for rows.Next() {
		a, err := scanAdminOAuthApp(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	return out, total, rows.Err()
}

func (r *oauthAdminRepository) GetApp(ctx context.Context, id uuid.UUID) (*models.AdminOAuthApp, error) {
	a, err := scanAdminOAuthApp(r.db.QueryRow(ctx, adminOAuthAppSelect+` WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

func (r *oauthAdminRepository) Suspend(ctx context.Context, id, adminID uuid.UUID, reason string) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE oauth_applications SET suspended_at = now(), suspended_reason = $2, suspended_by = $3, updated_at = now()
		WHERE id = $1 AND suspended_at IS NULL`, id, reason, adminID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *oauthAdminRepository) Unsuspend(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE oauth_applications SET suspended_at = NULL, suspended_reason = '', suspended_by = NULL, updated_at = now()
		WHERE id = $1`, id)
	return err
}

func (r *oauthAdminRepository) RevokeAllGrants(ctx context.Context, id uuid.UUID) (int64, error) {
	tag, err := r.db.Exec(ctx, `UPDATE oauth_access_grants SET revoked_at = now() WHERE application_id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *oauthAdminRepository) ClearLogo(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE oauth_applications SET logo_url = '', updated_at = now() WHERE id = $1`, id)
	return err
}

func (r *oauthAdminRepository) suspendWhere(ctx context.Context, cond string, arg, adminID uuid.UUID, reason string) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE oauth_applications SET suspended_at = now(), suspended_reason = $2, suspended_by = $3, updated_at = now()
		WHERE `+cond+` = $1 AND suspended_at IS NULL
		RETURNING id`, arg, reason, adminID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *oauthAdminRepository) SuspendOrgApps(ctx context.Context, orgID, adminID uuid.UUID, reason string) ([]uuid.UUID, error) {
	return r.suspendWhere(ctx, "organization_id", orgID, adminID, reason)
}

func (r *oauthAdminRepository) SuspendCreatorApps(ctx context.Context, userID, adminID uuid.UUID, reason string) ([]uuid.UUID, error) {
	return r.suspendWhere(ctx, "created_by", userID, adminID, reason)
}

const developerBlockSelect = `
	SELECT b.id, b.organization_id, COALESCE(o.name, ''), b.user_id, COALESCE(u.email, ''), b.reason, COALESCE(a.email, ''), b.created_at
	FROM oauth_developer_blocks b
	LEFT JOIN organizations o ON o.id = b.organization_id
	LEFT JOIN users u ON u.id = b.user_id
	LEFT JOIN users a ON a.id = b.blocked_by`

func scanDeveloperBlock(row pgx.Row) (*models.OAuthDeveloperBlock, error) {
	var b models.OAuthDeveloperBlock
	if err := row.Scan(&b.ID, &b.OrganizationID, &b.OrganizationName, &b.UserID, &b.UserEmail, &b.Reason, &b.BlockedByEmail, &b.CreatedAt); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *oauthAdminRepository) ListBlocks(ctx context.Context) ([]models.OAuthDeveloperBlock, error) {
	rows, err := r.db.Query(ctx, developerBlockSelect+` ORDER BY b.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OAuthDeveloperBlock{}
	for rows.Next() {
		b, err := scanDeveloperBlock(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (r *oauthAdminRepository) CreateBlock(ctx context.Context, orgID, userID *uuid.UUID, adminID uuid.UUID, reason string) (*models.OAuthDeveloperBlock, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO oauth_developer_blocks (organization_id, user_id, reason, blocked_by)
		VALUES ($1, $2, $3, $4) RETURNING id`, orgID, userID, reason, adminID).Scan(&id)
	if isUniqueViolation(err) {
		return nil, ErrDeveloperBlockExists
	}
	if err != nil {
		return nil, err
	}
	return scanDeveloperBlock(r.db.QueryRow(ctx, developerBlockSelect+` WHERE b.id = $1`, id))
}

func (r *oauthAdminRepository) DeleteBlock(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM oauth_developer_blocks WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
