// The analytics payload of GET /forms/:id/stats. Mirrors models.FormStats.

export type FormStatsRange = "7d" | "30d" | "90d";

export interface FormStatsTotals {
    views: number;
    starts: number;
    submissions: number;
    /** submissions / starts, 0..1. */
    completion_rate: number;
    identified_visitors: number;
}

export interface FormStatsDay {
    date: string;
    views: number;
    starts: number;
    submissions: number;
}

export interface FormFunnelPage {
    page_index: number;
    title: string;
    /** Visitors whose furthest page reached this one. */
    reached: number;
    /** Of those, how many went on to submit. */
    completed_from: number;
}

export interface FormStatsBucket {
    key: string;
    count: number;
}

export interface FormIdentifiedVisitor {
    contact_id: string;
    name: string;
    email: string;
    last_seen: Date;
    furthest_page: number;
    completed: boolean;
    /** Campaign whose email brought this contact here, when known. */
    campaign?: string;
}

export interface FormStats {
    totals: FormStatsTotals;
    daily: FormStatsDay[];
    pages: FormFunnelPage[];
    sources: FormStatsBucket[];
    countries: FormStatsBucket[];
    devices: FormStatsBucket[];
    /** Views by the campaign that carried the personalized link. */
    campaigns: FormStatsBucket[];
    identified: FormIdentifiedVisitor[];
}
