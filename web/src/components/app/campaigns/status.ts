// One place every surface reads a campaign's status from, so a list row, a
// picker row and a badge never disagree (and none of them leaks the raw
// backend value like "paused_no_accounts" at the user).

// Collapse the backend's raw campaign statuses into the buckets a list
// filters/counts by. paused_no_accounts / paused_trial_expired are auto-pause
// variants, so they belong with "paused"; anything unknown reads as draft.
export function campaignStatusBucket(s?: string): "active" | "paused" | "completed" | "draft" {
    if (s === "active") return "active";
    if (s === "completed") return "completed";
    if (s && s.startsWith("paused")) return "paused";
    return "draft";
}

export const CAMPAIGN_STATUS_LABEL: Record<string, string> = {
    active: "running",
    paused: "paused",
    paused_no_accounts: "no accounts",
    paused_trial_expired: "trial expired",
    paused_guardrail: "auto-paused",
    paused_undeliverable: "needs verification",
    completed: "finished",
    draft: "draft",
};

// Single source of truth for a status's color — drives BOTH the leading mark
// and the right-side text label so they always agree. emerald = live/done,
// amber = paused/needs-attention, slate = not started.
const CAMPAIGN_STATUS_TONE: Record<string, string> = {
    active: "text-emerald-600",
    completed: "text-emerald-600",
    paused: "text-amber-600",
    paused_no_accounts: "text-amber-600",
    paused_trial_expired: "text-amber-600",
    paused_guardrail: "text-rose-600",
    paused_undeliverable: "text-amber-600",
    draft: "text-slate-500",
};

// An unmapped status still has to read as words, not an enum: underscores
// become spaces rather than surfacing "PAUSED_NO_ACCOUNTS".
export function campaignStatusLabel(status?: string): string {
    if (!status) return CAMPAIGN_STATUS_LABEL.draft;
    return CAMPAIGN_STATUS_LABEL[status] ?? status.replace(/_/g, " ");
}

export function campaignStatusTone(status: string): string {
    return CAMPAIGN_STATUS_TONE[status] ?? CAMPAIGN_STATUS_TONE.draft;
}

// The subset of a campaign the display helpers need, so list rows, pickers
// and the detail header can all pass their own shape.
export interface CampaignStatusSubject {
    status?: string;
    start_date?: Date | string | null;
    idle_since?: Date | string | null;
}

// An active continuous campaign with nothing left to send: still running,
// waiting for leads. Its own label and tone, so it never reads as stuck.
export function isIdleCampaign(c: Pick<CampaignStatusSubject, "status" | "idle_since">): boolean {
    return c.status === "active" && !!c.idle_since;
}

export const CAMPAIGN_IDLE_LABEL = "waiting for leads";
export const CAMPAIGN_IDLE_TONE = "text-sky-600";

export function campaignDisplayTone(c: CampaignStatusSubject): string {
    if (isIdleCampaign(c)) return CAMPAIGN_IDLE_TONE;
    return campaignStatusTone(c.status ?? "draft");
}

export function campaignDisplayLabel(c: CampaignStatusSubject): string {
    if (isIdleCampaign(c)) return CAMPAIGN_IDLE_LABEL;
    return campaignStatusLabel(c.status ?? "draft");
}
