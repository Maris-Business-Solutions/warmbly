-- The campaign send-plan read endpoint used to run the whole planner walk on
-- the request: on a campaign of tens of thousands of leads the lead-supply and
-- history reads took tens of seconds and intermittently timed out. The walk now
-- runs in a background loop and writes its result here, so a read is a single
-- row fetch regardless of campaign size. One row per campaign, upserted; the
-- plan is a derived blob read whole and never filtered in SQL.
CREATE TABLE campaign_send_plan_snapshots (
    campaign_id     uuid PRIMARY KEY REFERENCES campaigns (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    -- The UTC budget day the plan counts. A snapshot from an earlier day is
    -- stale (daily counters reset at UTC midnight) and the read path refreshes it.
    day             text NOT NULL,
    -- The campaign version (id|status|updated_at) the plan was computed for, so
    -- an edit or a start/stop is seen as stale without reading the plan blob.
    version_key     text NOT NULL,
    -- The derived CampaignSendPlan, read whole and served as-is.
    plan            jsonb NOT NULL,
    computed_at     timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_campaign_send_plan_snapshots_org ON campaign_send_plan_snapshots (organization_id);
