package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/models"
)

// ErrAppListingSlugTaken is returned when another listing already holds the slug.
var ErrAppListingSlugTaken = errors.New("app listing slug taken")

// AppDirectoryRepository persists the community app directory. Developer reads
// and writes are scoped by the publishing organization; the browse queries are
// the one cross-organization read and return only public listing fields.
type AppDirectoryRepository interface {
	GetListing(ctx context.Context, orgID, appID uuid.UUID) (*models.AppListing, error)
	// SaveListing inserts or updates the app's listing. An update drops a
	// featured listing back to published and leaves a hidden one hidden.
	SaveListing(ctx context.Context, l *models.AppListing) error
	DeleteListing(ctx context.Context, orgID, appID uuid.UUID) error
	// Unfeature drops a featured listing back to published after its app's
	// public face (name, logo, website, scopes) changed.
	Unfeature(ctx context.Context, orgID, appID uuid.UUID) error

	// ListListed returns the listings shown in discovery: featured ones, and
	// published ones used by at least popularInstalls workspaces.
	ListListed(ctx context.Context, viewerOrgID uuid.UUID, popularInstalls, limit, offset int) ([]models.CommunityApp, int64, error)
	// GetPublished returns a listing reachable by its link, listed or not.
	GetPublished(ctx context.Context, viewerOrgID uuid.UUID, slug string, popularInstalls int) (*models.CommunityApp, error)

	AdminList(ctx context.Context, s *models.AdminAppListingSearch, popularInstalls int) ([]models.AdminAppListing, int64, error)
	AdminGet(ctx context.Context, appID uuid.UUID, popularInstalls int) (*models.AdminAppListing, error)
	SetStatus(ctx context.Context, appID uuid.UUID, status models.AppListingStatus, adminID uuid.UUID, note string) error
}

type appDirectoryRepository struct {
	db *pgxpool.Pool
}

func NewAppDirectoryRepository(db *pgxpool.Pool) AppDirectoryRepository {
	return &appDirectoryRepository{db: db}
}

const appListingCols = `l.application_id, l.organization_id, l.slug, l.tagline, l.description, l.category,
	l.install_url, l.support_url, l.privacy_url, l.status, l.status_note, l.status_at,
	l.submitted_at, l.created_at, l.updated_at`

func scanAppListing(row pgx.Row, l *models.AppListing, extra ...any) error {
	var status string
	dest := []any{&l.ApplicationID, &l.OrganizationID, &l.Slug, &l.Tagline, &l.Description, &l.Category,
		&l.InstallURL, &l.SupportURL, &l.PrivacyURL, &status, &l.StatusNote, &l.StatusAt,
		&l.SubmittedAt, &l.CreatedAt, &l.UpdatedAt}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return err
	}
	l.Status = models.AppListingStatus(status)
	return nil
}

func (r *appDirectoryRepository) GetListing(ctx context.Context, orgID, appID uuid.UUID) (*models.AppListing, error) {
	var l models.AppListing
	err := scanAppListing(r.db.QueryRow(ctx, `
		SELECT `+appListingCols+` FROM app_directory_listings l
		WHERE l.organization_id = $1 AND l.application_id = $2`, orgID, appID), &l)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *appDirectoryRepository) SaveListing(ctx context.Context, l *models.AppListing) error {
	now := time.Now().UTC()
	var status string
	err := r.db.QueryRow(ctx, `
		INSERT INTO app_directory_listings (application_id, organization_id, slug, tagline, description, category,
			install_url, support_url, privacy_url, status, submitted_at, created_at, updated_at)
		SELECT a.id, a.organization_id, $3, $4, $5, $6, $7, $8, $9, 'published', $10, $10, $10
		FROM oauth_applications a
		WHERE a.id = $2 AND a.organization_id = $1
		ON CONFLICT (application_id) DO UPDATE SET
			slug = EXCLUDED.slug, tagline = EXCLUDED.tagline, description = EXCLUDED.description,
			category = EXCLUDED.category, install_url = EXCLUDED.install_url,
			support_url = EXCLUDED.support_url, privacy_url = EXCLUDED.privacy_url,
			status = CASE WHEN app_directory_listings.status = 'featured' THEN 'published' ELSE app_directory_listings.status END,
			submitted_at = EXCLUDED.submitted_at, updated_at = EXCLUDED.updated_at
		WHERE app_directory_listings.organization_id = $1
		RETURNING status, submitted_at, created_at, updated_at, status_note, status_at`,
		l.OrganizationID, l.ApplicationID, l.Slug, l.Tagline, l.Description, l.Category,
		l.InstallURL, l.SupportURL, l.PrivacyURL, now,
	).Scan(&status, &l.SubmittedAt, &l.CreatedAt, &l.UpdatedAt, &l.StatusNote, &l.StatusAt)
	if isUniqueViolation(err) {
		return ErrAppListingSlugTaken
	}
	if err != nil {
		return err
	}
	l.Status = models.AppListingStatus(status)
	return nil
}

// DeleteListing unpublishes the app. A hidden listing stays, so an operator's
// decision is not undone by unpublishing and publishing again.
func (r *appDirectoryRepository) DeleteListing(ctx context.Context, orgID, appID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM app_directory_listings WHERE organization_id = $1 AND application_id = $2 AND status <> 'hidden'`, orgID, appID)
	return err
}

func (r *appDirectoryRepository) Unfeature(ctx context.Context, orgID, appID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE app_directory_listings
		SET status = 'published', submitted_at = now(), updated_at = now()
		WHERE organization_id = $1 AND application_id = $2 AND status = 'featured'`, orgID, appID)
	return err
}

// qualifiedInstallsSQL counts the workspaces that may list an app on installs
// alone: not the publisher's, and old enough not to be made for the purpose.
var qualifiedInstallsSQL = `SELECT count(DISTINCT g.organization_id) FROM oauth_access_grants g
	JOIN organizations og ON og.id = g.organization_id
	WHERE g.application_id = a.id AND g.revoked_at IS NULL AND g.organization_id <> l.organization_id
		AND og.created_at <= now() - make_interval(days => ` + strconv.Itoa(config.AppDirectoryInstallOrgMinAgeDays) + `)`

// communityAppCTE reads every reachable listing of an active app as the
// directory shows it. $1 is the viewing organization (for its installed flag
// only) and $2 the install count that lists a published app.
var communityAppCTE = `
	WITH c AS (
		SELECT l.application_id, l.slug, a.name, l.tagline, l.description, l.category, a.logo_url, a.website_url,
			l.install_url, l.support_url, l.privacy_url, COALESCE(o.name, '') AS developer, a.scopes, l.status,
			(SELECT count(DISTINCT g.organization_id) FROM oauth_access_grants g
				WHERE g.application_id = a.id AND g.revoked_at IS NULL)::int AS installs,
			(` + qualifiedInstallsSQL + `)::int AS qualified_installs,
			EXISTS (SELECT 1 FROM oauth_access_grants g
				WHERE g.application_id = a.id AND g.organization_id = $1 AND g.revoked_at IS NULL) AS installed,
			l.created_at
		FROM app_directory_listings l
		JOIN oauth_applications a ON a.id = l.application_id
		LEFT JOIN organizations o ON o.id = l.organization_id
		WHERE l.status IN ('published', 'featured') AND a.status = 'active' AND a.suspended_at IS NULL
	), listed AS (
		SELECT c.*, (c.status = 'featured' OR c.qualified_installs >= $2::int) AS is_listed FROM c
	)`

const communityAppCols = `application_id, slug, name, tagline, description, category, logo_url, website_url,
	install_url, support_url, privacy_url, developer, scopes, status, is_listed, installs, installed, created_at`

func scanCommunityApp(row pgx.Row) (*models.CommunityApp, error) {
	var app models.CommunityApp
	var scopes int64
	var status string
	if err := row.Scan(&app.ApplicationID, &app.Slug, &app.Name, &app.Tagline, &app.Description, &app.Category, &app.LogoURL, &app.WebsiteURL,
		&app.InstallURL, &app.SupportURL, &app.PrivacyURL, &app.Developer, &scopes, &status, &app.Listed,
		&app.Installs, &app.Installed, &app.PublishedAt); err != nil {
		return nil, err
	}
	app.Scopes = uint64(scopes)
	app.Permissions = models.PermissionsIn(app.Scopes)
	app.Status = models.AppListingStatus(status)
	return &app, nil
}

func (r *appDirectoryRepository) ListListed(ctx context.Context, viewerOrgID uuid.UUID, popularInstalls, limit, offset int) ([]models.CommunityApp, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, communityAppCTE+` SELECT count(*) FROM listed WHERE is_listed`,
		viewerOrgID, popularInstalls).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, communityAppCTE+`
		SELECT `+communityAppCols+` FROM listed WHERE is_listed
		ORDER BY (status = 'featured') DESC, installs DESC, lower(name), application_id
		LIMIT $3 OFFSET $4`, viewerOrgID, popularInstalls, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.CommunityApp{}
	for rows.Next() {
		app, err := scanCommunityApp(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *app)
	}
	return out, total, rows.Err()
}

func (r *appDirectoryRepository) GetPublished(ctx context.Context, viewerOrgID uuid.UUID, slug string, popularInstalls int) (*models.CommunityApp, error) {
	app, err := scanCommunityApp(r.db.QueryRow(ctx, communityAppCTE+`
		SELECT `+communityAppCols+` FROM listed WHERE slug = $3`, viewerOrgID, popularInstalls, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return app, err
}

// adminAppListingSelect: $1 is the install count that lists a published app.
var adminAppListingSelect = `
	SELECT ` + appListingCols + `, a.name, a.logo_url, a.website_url, a.scopes,
		CASE WHEN a.suspended_at IS NOT NULL THEN 'suspended' ELSE a.status END,
		COALESCE(o.name, ''), x.installs, (l.status = 'featured' OR (l.status = 'published' AND (` + qualifiedInstallsSQL + `) >= $1::int)),
		l.status_by, COALESCE(u.email, '')
	FROM app_directory_listings l
	JOIN oauth_applications a ON a.id = l.application_id
	LEFT JOIN organizations o ON o.id = l.organization_id
	LEFT JOIN users u ON u.id = l.status_by
	CROSS JOIN LATERAL (SELECT count(DISTINCT g.organization_id)::int AS installs FROM oauth_access_grants g
		WHERE g.application_id = a.id AND g.revoked_at IS NULL) x`

func scanAdminAppListing(row pgx.Row) (*models.AdminAppListing, error) {
	var out models.AdminAppListing
	var scopes int64
	if err := scanAppListing(row, &out.AppListing, &out.Name, &out.LogoURL, &out.WebsiteURL, &scopes, &out.AppStatus,
		&out.OrganizationName, &out.Installs, &out.Listed, &out.StatusBy, &out.StatusByEmail); err != nil {
		return nil, err
	}
	out.Scopes = uint64(scopes)
	out.Permissions = models.PermissionsIn(out.Scopes)
	return &out, nil
}

// appListingFilter builds the admin list's WHERE clause with placeholders
// numbered from first, so the count and the page can each start where they need.
func appListingFilter(s *models.AdminAppListingSearch, first int) (string, []any) {
	where := []string{"TRUE"}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return "$" + itoa(first+len(args)-1)
	}
	if s.Status != "" {
		where = append(where, "l.status = "+next(s.Status))
	}
	if q := strings.TrimSpace(s.Q); q != "" {
		n := next("%" + escapeLike(q) + "%")
		where = append(where, "(a.name ILIKE "+n+" OR l.slug ILIKE "+n+" OR o.name ILIKE "+n+" OR l.install_url ILIKE "+n+")")
	}
	return strings.Join(where, " AND "), args
}

func (r *appDirectoryRepository) AdminList(ctx context.Context, s *models.AdminAppListingSearch, popularInstalls int) ([]models.AdminAppListing, int64, error) {
	countCond, countArgs := appListingFilter(s, 1)
	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM app_directory_listings l
		JOIN oauth_applications a ON a.id = l.application_id
		LEFT JOIN organizations o ON o.id = l.organization_id
		WHERE `+countCond, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	cond, filterArgs := appListingFilter(s, 2)
	args := append([]any{popularInstalls}, filterArgs...)
	args = append(args, s.Limit, s.Offset)
	rows, err := r.db.Query(ctx, adminAppListingSelect+`
		WHERE `+cond+`
		ORDER BY (l.status = 'featured') DESC, x.installs DESC, l.submitted_at DESC, l.application_id
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.AdminAppListing{}
	for rows.Next() {
		item, err := scanAdminAppListing(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, rows.Err()
}

func (r *appDirectoryRepository) AdminGet(ctx context.Context, appID uuid.UUID, popularInstalls int) (*models.AdminAppListing, error) {
	item, err := scanAdminAppListing(r.db.QueryRow(ctx, adminAppListingSelect+` WHERE l.application_id = $2`, popularInstalls, appID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *appDirectoryRepository) SetStatus(ctx context.Context, appID uuid.UUID, status models.AppListingStatus, adminID uuid.UUID, note string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE app_directory_listings
		SET status = $2, status_note = $3, status_by = $4, status_at = now(), updated_at = now()
		WHERE application_id = $1`, appID, string(status), note, adminID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
