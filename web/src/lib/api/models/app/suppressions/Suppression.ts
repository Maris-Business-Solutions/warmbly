// One entry on the workspace suppression list, mirroring
// models.SuppressedRecipient. A "domain" entry keeps the bare host in `email`.
export type SuppressionKind = "email" | "domain";
export type SuppressionSource = "bounce" | "complaint" | "unsubscribe" | "manual" | "import";

export default interface Suppression {
    id: string;
    organization_id: string;
    email: string;
    kind: SuppressionKind;
    reason: string;
    source: SuppressionSource | string;
    campaign_id?: string | null;
    expires_at?: Date | null;
    metadata?: Record<string, unknown>;
    created_at: Date;
    updated_at: Date;
}

export interface SuppressionListResult {
    data: Suppression[];
    pagination: { next_cursor: string | null; has_more: boolean };
}

export interface AddSuppressionsRequest {
    entries: { value: string; reason?: string }[];
    reason?: string;
}

export interface AddSuppressionsResult {
    added: number;
    skipped: string[];
}

export const SOURCE_LABEL: Record<string, string> = {
    bounce: "Bounced",
    complaint: "Spam complaint",
    unsubscribe: "Unsubscribed",
    manual: "Added by hand",
    import: "Imported",
};

// Entries the recipient made themselves (or the mail system recorded) are
// warned about before removal; a hand-added one is the team's own call.
export function recipientTriggered(s: Suppression): boolean {
    return s.source === "unsubscribe" || s.source === "complaint" || s.source === "bounce";
}
