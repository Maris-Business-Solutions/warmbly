package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/warmbly/warmbly/internal/infrastructure/db"
	"github.com/warmbly/warmbly/internal/models"
)

// CampaignSendPlanSnapshot is one campaign's precomputed send plan for a budget
// day, written by the background snapshotter and served by the read endpoint.
type CampaignSendPlanSnapshot struct {
	CampaignID     uuid.UUID
	OrganizationID uuid.UUID
	// Day is the UTC budget day the plan counts (now.UTC() "2006-01-02").
	Day string
	// VersionKey is the campaign version the plan was computed for, so an edit
	// or a start/stop is seen as stale without parsing the plan blob.
	VersionKey string
	Plan       *models.CampaignSendPlan
	ComputedAt time.Time
}

// CampaignSendPlanSnapshotRepository stores the latest send plan per campaign.
type CampaignSendPlanSnapshotRepository interface {
	// Get returns the campaign's snapshot, or nil when none has been written.
	// Scoped by organization so a read cannot cross tenants.
	Get(ctx context.Context, orgID, campaignID uuid.UUID) (*CampaignSendPlanSnapshot, error)
	// Upsert writes (or replaces) the campaign's snapshot.
	Upsert(ctx context.Context, snap *CampaignSendPlanSnapshot) error
}

type campaignSendPlanSnapshotRepository struct {
	db *db.DB
}

// NewCampaignSendPlanSnapshotRepository wires the snapshot store.
func NewCampaignSendPlanSnapshotRepository(db *db.DB) CampaignSendPlanSnapshotRepository {
	return &campaignSendPlanSnapshotRepository{db: db}
}

func (r *campaignSendPlanSnapshotRepository) Get(ctx context.Context, orgID, campaignID uuid.UUID) (*CampaignSendPlanSnapshot, error) {
	var (
		snap CampaignSendPlanSnapshot
		raw  []byte
	)
	err := r.db.QueryRow(ctx, `
		SELECT campaign_id, organization_id, day, version_key, plan, computed_at
		FROM campaign_send_plan_snapshots
		WHERE campaign_id = $1 AND organization_id = $2
	`, campaignID, orgID).Scan(&snap.CampaignID, &snap.OrganizationID, &snap.Day, &snap.VersionKey, &raw, &snap.ComputedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var plan models.CampaignSendPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, err
	}
	snap.Plan = &plan
	return &snap, nil
}

func (r *campaignSendPlanSnapshotRepository) Upsert(ctx context.Context, snap *CampaignSendPlanSnapshot) error {
	raw, err := json.Marshal(snap.Plan)
	if err != nil {
		return err
	}
	// The guard rejects a regression: a planner walk for an older version starts
	// earlier, so its computed_at is earlier, and it must not overwrite a newer
	// walk's snapshot that already landed. version_key is a hash and is not
	// ordered, so recency is taken from computed_at (stamped at walk start).
	_, err = r.db.Exec(ctx, `
		INSERT INTO campaign_send_plan_snapshots (campaign_id, organization_id, day, version_key, plan, computed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (campaign_id) DO UPDATE SET
			organization_id = EXCLUDED.organization_id,
			day = EXCLUDED.day,
			version_key = EXCLUDED.version_key,
			plan = EXCLUDED.plan,
			computed_at = EXCLUDED.computed_at,
			updated_at = NOW()
		WHERE campaign_send_plan_snapshots.computed_at <= EXCLUDED.computed_at
	`, snap.CampaignID, snap.OrganizationID, snap.Day, snap.VersionKey, raw, snap.ComputedAt)
	return err
}
