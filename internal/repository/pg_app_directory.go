package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/models"
)

// ErrAppListingSlugTaken is returned when another listing already holds the slug.
var ErrAppListingSlugTaken = errors.New("app listing slug taken")

// AppDirectoryRepository persists the community app directory. Developer reads
// and writes are scoped by the publishing organization; the browse queries are
// the one cross-organization read and return only public listing fields.
type AppDirectoryRepository interface {
	GetListing(ctx context.Context, orgID, appID uuid.UUID) (*models.AppListing, error)
	// SaveListing inserts or updates the app's listing. Any save puts it back in
	// the review queue as unverified.
	SaveListing(ctx context.Context, l *models.AppListing) error
	DeleteListing(ctx context.Context, orgID, appID uuid.UUID) error
	// Unverify returns a verified listing to the queue after its app's public
	// face (name, logo, website, scopes) changed.
	Unverify(ctx context.Context, orgID, appID uuid.UUID) error

	// ListVerified returns the verified listings of active apps, most installed first.
	ListVerified(ctx context.Context, viewerOrgID uuid.UUID, limit, offset int) ([]models.CommunityApp, int64, error)
	// GetPublished returns a listing reachable by its link: verified or
	// unverified, of an active app. Rejected listings are not reachable.
	GetPublished(ctx context.Context, viewerOrgID uuid.UUID, slug string) (*models.CommunityApp, error)

	AdminList(ctx context.Context, s *models.AdminAppListingSearch) ([]models.AdminAppListing, int64, error)
	AdminGet(ctx context.Context, appID uuid.UUID) (*models.AdminAppListing, error)
	SetVerification(ctx context.Context, appID uuid.UUID, v models.AppListingVerification, adminID uuid.UUID, note string) error
}

type appDirectoryRepository struct {
	db *pgxpool.Pool
}

func NewAppDirectoryRepository(db *pgxpool.Pool) AppDirectoryRepository {
	return &appDirectoryRepository{db: db}
}

const appListingCols = `l.application_id, l.organization_id, l.slug, l.tagline, l.description, l.category,
	l.install_url, l.support_url, l.privacy_url, l.verification, l.review_note, l.reviewed_at,
	l.submitted_at, l.created_at, l.updated_at`

func scanAppListing(row pgx.Row, l *models.AppListing, extra ...any) error {
	var verification string
	dest := []any{&l.ApplicationID, &l.OrganizationID, &l.Slug, &l.Tagline, &l.Description, &l.Category,
		&l.InstallURL, &l.SupportURL, &l.PrivacyURL, &verification, &l.ReviewNote, &l.ReviewedAt,
		&l.SubmittedAt, &l.CreatedAt, &l.UpdatedAt}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return err
	}
	l.Verification = models.AppListingVerification(verification)
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
	var verification string
	err := r.db.QueryRow(ctx, `
		INSERT INTO app_directory_listings (application_id, organization_id, slug, tagline, description, category,
			install_url, support_url, privacy_url, verification, review_note, submitted_at, created_at, updated_at)
		SELECT a.id, a.organization_id, $3, $4, $5, $6, $7, $8, $9, 'unverified', '', $10, $10, $10
		FROM oauth_applications a
		WHERE a.id = $2 AND a.organization_id = $1
		ON CONFLICT (application_id) DO UPDATE SET
			slug = EXCLUDED.slug, tagline = EXCLUDED.tagline, description = EXCLUDED.description,
			category = EXCLUDED.category, install_url = EXCLUDED.install_url,
			support_url = EXCLUDED.support_url, privacy_url = EXCLUDED.privacy_url,
			verification = 'unverified', submitted_at = EXCLUDED.submitted_at, updated_at = EXCLUDED.updated_at
		WHERE app_directory_listings.organization_id = $1
		RETURNING verification, submitted_at, created_at, updated_at, review_note, reviewed_at`,
		l.OrganizationID, l.ApplicationID, l.Slug, l.Tagline, l.Description, l.Category,
		l.InstallURL, l.SupportURL, l.PrivacyURL, now,
	).Scan(&verification, &l.SubmittedAt, &l.CreatedAt, &l.UpdatedAt, &l.ReviewNote, &l.ReviewedAt)
	if isUniqueViolation(err) {
		return ErrAppListingSlugTaken
	}
	if err != nil {
		return err
	}
	l.Verification = models.AppListingVerification(verification)
	return nil
}

func (r *appDirectoryRepository) DeleteListing(ctx context.Context, orgID, appID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM app_directory_listings WHERE organization_id = $1 AND application_id = $2`, orgID, appID)
	return err
}

func (r *appDirectoryRepository) Unverify(ctx context.Context, orgID, appID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		UPDATE app_directory_listings
		SET verification = 'unverified', submitted_at = now(), updated_at = now()
		WHERE organization_id = $1 AND application_id = $2 AND verification = 'verified'`, orgID, appID)
	return err
}

// communityAppSelect reads a listing as the directory shows it; $1 is the
// viewing organization, used only for its own installed flag.
const communityAppSelect = `
	SELECT l.application_id, l.slug, a.name, l.tagline, l.description, l.category, a.logo_url, a.website_url,
		l.install_url, l.support_url, l.privacy_url, COALESCE(o.name, ''), a.scopes, l.verification,
		(SELECT count(DISTINCT g.organization_id) FROM oauth_access_grants g
			WHERE g.application_id = a.id AND g.revoked_at IS NULL)::int AS installs,
		EXISTS (SELECT 1 FROM oauth_access_grants g
			WHERE g.application_id = a.id AND g.organization_id = $1 AND g.revoked_at IS NULL) AS installed,
		l.created_at
	FROM app_directory_listings l
	JOIN oauth_applications a ON a.id = l.application_id
	LEFT JOIN organizations o ON o.id = l.organization_id`

func scanCommunityApp(row pgx.Row) (*models.CommunityApp, error) {
	var app models.CommunityApp
	var scopes int64
	var verification string
	if err := row.Scan(&app.ApplicationID, &app.Slug, &app.Name, &app.Tagline, &app.Description, &app.Category, &app.LogoURL, &app.WebsiteURL,
		&app.InstallURL, &app.SupportURL, &app.PrivacyURL, &app.Developer, &scopes, &verification,
		&app.Installs, &app.Installed, &app.PublishedAt); err != nil {
		return nil, err
	}
	app.Scopes = uint64(scopes)
	app.Permissions = models.PermissionsIn(app.Scopes)
	app.Verification = models.AppListingVerification(verification)
	return &app, nil
}

func (r *appDirectoryRepository) ListVerified(ctx context.Context, viewerOrgID uuid.UUID, limit, offset int) ([]models.CommunityApp, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM app_directory_listings l
		JOIN oauth_applications a ON a.id = l.application_id
		WHERE l.verification = 'verified' AND a.status = 'active'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, communityAppSelect+`
		WHERE l.verification = 'verified' AND a.status = 'active'
		ORDER BY installs DESC, lower(a.name), l.application_id
		LIMIT $2 OFFSET $3`, viewerOrgID, limit, offset)
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

func (r *appDirectoryRepository) GetPublished(ctx context.Context, viewerOrgID uuid.UUID, slug string) (*models.CommunityApp, error) {
	app, err := scanCommunityApp(r.db.QueryRow(ctx, communityAppSelect+`
		WHERE l.slug = $2 AND l.verification IN ('verified', 'unverified') AND a.status = 'active'`, viewerOrgID, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return app, err
}

const adminAppListingSelect = `
	SELECT ` + appListingCols + `, a.name, a.logo_url, a.website_url, a.scopes, a.status,
		COALESCE(o.name, ''), l.reviewed_by, COALESCE(u.email, ''),
		(SELECT count(DISTINCT g.organization_id) FROM oauth_access_grants g
			WHERE g.application_id = a.id AND g.revoked_at IS NULL)::int
	FROM app_directory_listings l
	JOIN oauth_applications a ON a.id = l.application_id
	LEFT JOIN organizations o ON o.id = l.organization_id
	LEFT JOIN users u ON u.id = l.reviewed_by`

func scanAdminAppListing(row pgx.Row) (*models.AdminAppListing, error) {
	var out models.AdminAppListing
	var scopes int64
	if err := scanAppListing(row, &out.AppListing, &out.Name, &out.LogoURL, &out.WebsiteURL, &scopes, &out.AppStatus,
		&out.OrganizationName, &out.ReviewedBy, &out.ReviewedByEmail, &out.Installs); err != nil {
		return nil, err
	}
	out.Scopes = uint64(scopes)
	out.Permissions = models.PermissionsIn(out.Scopes)
	return &out, nil
}

func (r *appDirectoryRepository) AdminList(ctx context.Context, s *models.AdminAppListingSearch) ([]models.AdminAppListing, int64, error) {
	where := []string{"TRUE"}
	args := []any{}
	if s.Verification != "" {
		args = append(args, s.Verification)
		where = append(where, "l.verification = $"+itoa(len(args)))
	}
	if q := strings.TrimSpace(s.Q); q != "" {
		args = append(args, "%"+escapeLike(q)+"%")
		n := itoa(len(args))
		where = append(where, "(a.name ILIKE $"+n+" OR l.slug ILIKE $"+n+" OR o.name ILIKE $"+n+" OR l.install_url ILIKE $"+n+")")
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM app_directory_listings l
		JOIN oauth_applications a ON a.id = l.application_id
		LEFT JOIN organizations o ON o.id = l.organization_id
		WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, s.Limit, s.Offset)
	rows, err := r.db.Query(ctx, adminAppListingSelect+`
		WHERE `+cond+`
		ORDER BY (l.verification = 'unverified') DESC, l.submitted_at ASC, l.application_id
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

func (r *appDirectoryRepository) AdminGet(ctx context.Context, appID uuid.UUID) (*models.AdminAppListing, error) {
	item, err := scanAdminAppListing(r.db.QueryRow(ctx, adminAppListingSelect+` WHERE l.application_id = $1`, appID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *appDirectoryRepository) SetVerification(ctx context.Context, appID uuid.UUID, v models.AppListingVerification, adminID uuid.UUID, note string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE app_directory_listings
		SET verification = $2, review_note = $3, reviewed_by = $4, reviewed_at = now(), updated_at = now()
		WHERE application_id = $1`, appID, string(v), note, adminID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
