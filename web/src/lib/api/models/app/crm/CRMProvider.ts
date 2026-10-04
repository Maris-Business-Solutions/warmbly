// A workspace runs one CRM: Warmbly's own, or a connected one (HubSpot or
// Pipedrive). In provider mode deals, tasks, notes and pipelines are the
// provider's records, mirrored locally and written through on every change.

export type CRMProviderName = "native" | "hubspot" | "pipedrive";

// Where a record lives outside Warmbly. Set on deals, tasks, notes and
// pipelines read in provider mode.
export interface CRMExternalRef {
    provider: CRMProviderName;
    external_id: string;
    url?: string;
    synced_at: Date;
    // The provider's owner, when that owner is not a workspace member.
    owner_name?: string;
}

export interface CRMActivityLog {
    sent: boolean;
    replies: boolean;
    bounces: boolean;
    unsubscribes: boolean;
    opens: boolean;
    clicks: boolean;
    meetings: boolean;
}

export interface CRMReplyOutcome {
    lead_status: string;
    // A lifecycle stage in HubSpot, a person label id in Pipedrive.
    lifecycle_stage: string;
    create_deal: boolean;
    // Adds the person to Pipedrive's Leads Inbox (Pipedrive only).
    create_lead?: boolean;
    deal_pipeline_id?: string;
    deal_stage_id?: string;
}

export interface CRMExitRules {
    deal_created: boolean;
    lifecycle_stages: string[];
    opted_out: boolean;
}

export interface CRMEnrollmentGuards {
    skip_lifecycle_stages: string[];
    skip_open_deals: boolean;
    skip_other_owners: boolean;
    skip_opted_out: boolean;
}

export type CRMFieldDirection = "push" | "pull" | "both";

export interface CRMProviderConfig {
    activity: CRMActivityLog;
    create_contacts: boolean;
    create_companies: boolean;
    write_properties: boolean;
    positive_reply: CRMReplyOutcome;
    exit_rules: CRMExitRules;
    guards: CRMEnrollmentGuards;
    // Provider pipeline ids to mirror; empty means every pipeline.
    deal_pipelines: string[];
    display_properties: string[];
    // Warmbly field (first_name, company, custom:x) -> provider property.
    field_map: Record<string, string>;
    field_direction: Record<string, CRMFieldDirection>;
    // The provider these choices were made for.
    for?: CRMProviderName;
}

export interface CRMAccount {
    external_id: string;
    name: string;
    app_url: string;
    status: string;
    health: string;
    // Permissions the connection lacks; non-empty means "reconnect".
    missing_scopes?: string[];
}

export interface CRMSettings {
    organization_id: string;
    provider: CRMProviderName;
    connection_id?: string;
    config: CRMProviderConfig;
    setup_completed_at?: Date;
    updated_at: Date;
    account?: CRMAccount;
}

export interface UpdateCRMSettings {
    provider?: CRMProviderName;
    connection_id?: string;
    config?: CRMProviderConfig;
    complete_setup?: boolean;
}

export interface CRMOption {
    value: string;
    label: string;
}

export interface CRMProperty {
    name: string;
    label: string;
    type: string;
    group_name: string;
    read_only: boolean;
}

export interface CRMMetadata {
    lifecycle_stages: CRMOption[];
    lead_statuses: CRMOption[];
    task_types: CRMOption[];
    properties: CRMProperty[];
    pipelines: CRMOption[];
}

export interface CRMOwner {
    external_id: string;
    email: string;
    first_name: string;
    last_name: string;
    user_id?: string;
    user_pinned: boolean;
    archived: boolean;
}

export interface CRMCompanyRef {
    external_id: string;
    name: string;
    domain?: string;
    url?: string;
}

export interface CRMPropertyView {
    name: string;
    label: string;
    value: string;
}

// The provider's side of one contact, for the inbox panel and the contact drawer.
export interface CRMContactView {
    provider: CRMProviderName;
    linked: boolean;
    external_id?: string;
    url?: string;
    owner?: CRMOwner;
    lifecycle_stage?: CRMOption;
    lead_status?: CRMOption;
    company?: CRMCompanyRef;
    opted_out: boolean;
    properties: CRMPropertyView[];
    synced_at?: Date;
}

export interface UpdateCRMContact {
    owner_external_id?: string;
    lifecycle_stage?: string;
    lead_status?: string;
}

export interface CRMSyncJob {
    id: string;
    organization_id: string;
    provider: CRMProviderName;
    kind: string;
    subject: string;
    status: "pending" | "running" | "done" | "failed";
    attempts: number;
    next_attempt_at: Date;
    last_error?: string;
    created_at: Date;
    updated_at: Date;
    finished_at?: Date;
}

export interface CRMSyncCursor {
    object_type: string;
    cursor_at?: Date;
    last_run_at?: Date;
    last_error?: string;
}

export interface CRMSyncHealth {
    pending: number;
    failed: number;
    done_24h: number;
    last_synced_at?: Date;
    cursors: CRMSyncCursor[];
    failures: CRMSyncJob[];
    counts: {
        contacts: number;
        deals: number;
        tasks: number;
        pipelines: number;
        owners: number;
    };
}

export interface CRMList {
    external_id: string;
    name: string;
    size: number;
    dynamic: boolean;
    updated_at?: string;
}

export interface CRMListsResult {
    data: CRMList[];
    pagination: {
        has_more: boolean;
        next_cursor?: string | null;
    };
}

export interface CRMImportRequest {
    list_id: string;
    apply_guards: boolean;
}

export interface CRMImportSkip {
    reason: string;
    label: string;
    count: number;
}

export interface CRMImportPreview {
    list_name: string;
    total: number;
    included: number;
    skipped: CRMImportSkip[];
    sample: { email: string; first_name: string; last_name: string; company: string }[];
    truncated: boolean;
}

export interface CRMImportResult {
    import_id: string;
    preview: CRMImportPreview;
}

export interface CRMBackfillPreview {
    deals: number;
    tasks: number;
    notes: number;
}

export interface CRMBackfillRequest {
    deals: boolean;
    tasks: boolean;
    notes: boolean;
}
