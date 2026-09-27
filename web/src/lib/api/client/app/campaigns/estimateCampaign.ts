import Request from "../../Request";

// Wire shape for POST /campaigns-estimate: an audience (segments) against a
// sender pool (tags, or every active mailbox when empty) under a per-mailbox
// daily limit and a weekday mask. Read-only; nothing is created.
export interface CampaignEstimateInput {
    segment_ids: string[];
    email_tag_ids?: string[];
    daily_limit?: number;
    days?: number;
    timezone?: string;
    // RFC 3339. Omit to start now.
    start_date?: string;
}

export interface CampaignEstimateResult {
    recipients: number;
    mailboxes: number;
    // Pool ceiling per sending day under the campaign limit.
    daily_capacity: number;
    // What the pool can still send today.
    remaining_today: number;
    // Null when the pool has no capacity or the audience is empty.
    sending_days: number | null;
    estimated_finish_at: Date | null;
}

export default async function estimateCampaign(input: CampaignEstimateInput): Promise<CampaignEstimateResult> {
    return await Request<CampaignEstimateResult>({
        method: "POST",
        url: `/campaigns-estimate`,
        data: input,
        authorization: true,
    });
}
