-- Extra recipients copied on every email one campaign sends one lead, so two
-- people at the same company can be reached in one thread instead of two
-- parallel sequences (issue #731). The copies are contacts, not free text, so
-- suppression, bounces and verification apply to them exactly as to a lead.
CREATE TABLE campaign_lead_cc (
    campaign_id   uuid NOT NULL,
    contact_id    uuid NOT NULL,
    cc_contact_id uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    position      smallint NOT NULL DEFAULT 0,
    -- A bounce attributed to this copy on this lead's thread. It is dropped
    -- from later emails whatever the workspace's auto-suppress setting says.
    bounced_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (campaign_id, contact_id, cc_contact_id),
    FOREIGN KEY (campaign_id, contact_id) REFERENCES campaign_leads (campaign_id, contact_id) ON DELETE CASCADE,
    CONSTRAINT campaign_lead_cc_not_self CHECK (cc_contact_id <> contact_id)
);

-- Serves the contact FK's cascade and "is this contact copied on a lead here".
CREATE INDEX idx_campaign_lead_cc_cc ON campaign_lead_cc (cc_contact_id, campaign_id);

-- A contact copied on another lead's thread is reached there, so their own
-- lead in the same campaign is held with source 'cc' instead of starting a
-- second sequence.
ALTER TABLE public.campaign_leads DROP CONSTRAINT IF EXISTS campaign_leads_pause_source_check;
-- NOT VALID: 000231 validates it in its own transaction.
ALTER TABLE public.campaign_leads
    ADD CONSTRAINT campaign_leads_pause_source_check
    CHECK (pause_source IS NULL OR pause_source IN ('manual', 'out_of_office', 'inbox_tagging', 'cc')) NOT VALID;

-- Triggers rather than callers, so every path that enrols a lead (segments,
-- imports, contact edits, org transfer) holds a copied contact the same way.
CREATE FUNCTION campaign_lead_cc_hold() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- A member's own live pause outranks this; anything else is replaced.
        UPDATE campaign_leads cl
        SET paused_at = CASE
                WHEN cl.paused_at IS NOT NULL AND (cl.paused_until IS NULL OR cl.paused_until > NOW())
                THEN cl.paused_at ELSE NOW() END,
            paused_until = NULL,
            pause_reason = (SELECT c.email FROM contacts c WHERE c.id = NEW.contact_id),
            pause_source = 'cc'
        WHERE cl.campaign_id = NEW.campaign_id
          AND cl.contact_id = NEW.cc_contact_id
          AND NOT (cl.pause_source = 'manual' AND cl.paused_at IS NOT NULL
                   AND (cl.paused_until IS NULL OR cl.paused_until > NOW()));
        RETURN NEW;
    END IF;
    -- Released once no lead in the campaign copies them any more.
    UPDATE campaign_leads cl
    SET paused_at = NULL, paused_until = NULL, pause_reason = NULL, pause_source = NULL
    WHERE cl.campaign_id = OLD.campaign_id
      AND cl.contact_id = OLD.cc_contact_id
      AND cl.pause_source = 'cc'
      AND NOT EXISTS (
          SELECT 1 FROM campaign_lead_cc x
          WHERE x.campaign_id = OLD.campaign_id AND x.cc_contact_id = OLD.cc_contact_id
      );
    RETURN OLD;
END;
$$;

CREATE TRIGGER campaign_lead_cc_hold
    AFTER INSERT OR DELETE ON campaign_lead_cc
    FOR EACH ROW EXECUTE FUNCTION campaign_lead_cc_hold();

-- The other order: a contact already copied on a lead is enrolled later.
CREATE FUNCTION campaign_lead_enrol_cc_hold() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    lead_email text;
BEGIN
    SELECT c.email INTO lead_email
    FROM campaign_lead_cc x
    JOIN contacts c ON c.id = x.contact_id
    WHERE x.campaign_id = NEW.campaign_id AND x.cc_contact_id = NEW.contact_id
    ORDER BY x.created_at
    LIMIT 1;
    IF FOUND AND NEW.paused_at IS NULL THEN
        NEW.paused_at := NOW();
        NEW.paused_until := NULL;
        NEW.pause_reason := lead_email;
        NEW.pause_source := 'cc';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER campaign_lead_enrol_cc_hold
    BEFORE INSERT ON campaign_leads
    FOR EACH ROW EXECUTE FUNCTION campaign_lead_enrol_cc_hold();
