package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/models"
)

// ViewPreferencesRepository stores one member's saved layout per dashboard
// list per workspace.
type ViewPreferencesRepository interface {
	// Get returns nil, nil when the member has never saved this view.
	Get(ctx context.Context, userID, orgID uuid.UUID, view string) (*models.ViewPreferences, error)
	// Upsert writes the fields the update carries, keeps the rest, and returns
	// the row as saved.
	Upsert(ctx context.Context, userID, orgID uuid.UUID, view string, upd models.ViewPreferencesUpdate) (*models.ViewPreferences, error)
	Delete(ctx context.Context, userID, orgID uuid.UUID, view string) error
}

type viewPreferencesRepository struct {
	db *pgxpool.Pool
}

func NewViewPreferencesRepository(db *pgxpool.Pool) ViewPreferencesRepository {
	return &viewPreferencesRepository{db: db}
}

// scanPrefs reads one row's columns, sort, layout and updated_at.
func scanPrefs(view string, row pgx.Row) (*models.ViewPreferences, error) {
	var (
		raw       []byte
		sortBy    string
		reverse   bool
		layout    []byte
		updatedAt time.Time
	)
	if err := row.Scan(&raw, &sortBy, &reverse, &layout, &updatedAt); err != nil {
		return nil, err
	}
	prefs := &models.ViewPreferences{View: view, Columns: []string{}, UpdatedAt: &updatedAt}
	if models.ViewHasLayout(view) && len(layout) > 0 {
		prefs.Layout = layout
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &prefs.Columns); err != nil {
			return nil, err
		}
	}
	if sortBy != "" {
		prefs.Sort = &models.ViewSort{By: sortBy, Reverse: reverse}
	}
	return prefs, nil
}

func (r *viewPreferencesRepository) Get(ctx context.Context, userID, orgID uuid.UUID, view string) (*models.ViewPreferences, error) {
	prefs, err := scanPrefs(view, r.db.QueryRow(ctx, `
		SELECT columns, sort_by, sort_reverse, layout, updated_at
		FROM user_view_preferences
		WHERE user_id = $1 AND organization_id = $2 AND view = $3`,
		userID, orgID, view))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return prefs, err
}

func (r *viewPreferencesRepository) Upsert(ctx context.Context, userID, orgID uuid.UUID, view string, upd models.ViewPreferencesUpdate) (*models.ViewPreferences, error) {
	// A nil field binds NULL and COALESCE keeps the saved value, reading the
	// bound parameter rather than EXCLUDED: the VALUES row has already turned
	// that NULL into the default, so EXCLUDED would never be NULL.
	var raw []byte
	if upd.Columns != nil {
		cols := *upd.Columns
		if cols == nil {
			cols = []string{}
		}
		var err error
		if raw, err = json.Marshal(cols); err != nil {
			return nil, err
		}
	}
	var sortBy *string
	var reverse *bool
	if upd.Sort != nil {
		sortBy, reverse = &upd.Sort.By, &upd.Sort.Reverse
	}
	var layout []byte
	if upd.Layout != nil {
		layout = upd.Layout
	}
	return scanPrefs(view, r.db.QueryRow(ctx, `
		INSERT INTO user_view_preferences (user_id, organization_id, view, columns, sort_by, sort_reverse, layout, updated_at)
		VALUES ($1, $2, $3, COALESCE($4::jsonb, '[]'::jsonb), COALESCE($5::text, ''), COALESCE($6::boolean, false), COALESCE($7::jsonb, '{}'::jsonb), now())
		ON CONFLICT (user_id, organization_id, view) DO UPDATE
		SET columns = COALESCE($4::jsonb, user_view_preferences.columns),
		    sort_by = COALESCE($5::text, user_view_preferences.sort_by),
		    sort_reverse = COALESCE($6::boolean, user_view_preferences.sort_reverse),
		    layout = COALESCE($7::jsonb, user_view_preferences.layout),
		    updated_at = now()
		RETURNING columns, sort_by, sort_reverse, layout, updated_at`,
		userID, orgID, view, raw, sortBy, reverse, layout))
}

func (r *viewPreferencesRepository) Delete(ctx context.Context, userID, orgID uuid.UUID, view string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM user_view_preferences
		WHERE user_id = $1 AND organization_id = $2 AND view = $3`,
		userID, orgID, view)
	return err
}
