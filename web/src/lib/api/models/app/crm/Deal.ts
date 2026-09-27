export type DealStatus = "open" | "won" | "lost";

export default interface Deal {
    id: string;
    organization_id: string;
    pipeline_id: string;
    stage_id: string;
    contact_id?: string;
    name: string;
    value?: number;
    currency: string;
    status: DealStatus;
    expected_close_date?: Date;
    won_at?: Date;
    lost_at?: Date;
    lost_reason?: string;
    assigned_to?: string;
    // Attribution: the campaign + sender mailbox that produced the originating
    // reply. Nullable; editable. Lets won revenue trace back to outreach.
    campaign_id?: string;
    source_mailbox_id?: string;
    created_at: Date;
    updated_at: Date;

    // Optional joined fields the API may return
    contact?: {
        id: string;
        first_name: string;
        last_name: string;
        email: string;
        company: string;
    };
    stage?: {
        id: string;
        name: string;
        color: string;
        position: number;
    };
    campaign_name?: string;
}

// Create/update body: expected_close_date goes out as the RFC3339 string the form builds.
export type DealWrite = Partial<Omit<Deal, "expected_close_date">> & { expected_close_date?: string };

export interface DealsResult {
    data: Deal[];
    pagination: {
        has_more: boolean;
        next_cursor?: string | null;
    };
}
