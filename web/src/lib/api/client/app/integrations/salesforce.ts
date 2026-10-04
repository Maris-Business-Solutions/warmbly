// Native Salesforce sync endpoints, all scoped to one connection id.

import type {
    ContactSalesforcePanel,
    CreateSalesforceImportSourceInput,
    SalesforceActivityPage,
    SalesforceActivityStatus,
    SalesforceCampaign,
    SalesforceImportObject,
    SalesforceImportPreview,
    SalesforceImportSource,
    SalesforceListView,
    SalesforceMetadata,
    SalesforceOverview,
    SalesforceSettings,
    SalesforceSettingsResponse,
    SalesforceSourceKind,
    SalesforceUser,
    UpdateSalesforceImportSourceInput,
} from "@/lib/api/models/app/integrations/Salesforce";
import Request from "../../Request";

const base = (id: string) => `/integrations/salesforce/${id}`;

export async function getSalesforceOverview(id: string, checks = false): Promise<SalesforceOverview> {
    return await Request<SalesforceOverview>({
        method: "GET",
        url: `${base(id)}/overview`,
        params: checks ? { checks: 1 } : undefined,
        authorization: true,
    });
}

export async function getSalesforceSettings(id: string): Promise<SalesforceSettingsResponse> {
    return await Request<SalesforceSettingsResponse>({
        method: "GET",
        url: `${base(id)}/settings`,
        authorization: true,
    });
}

export async function updateSalesforceSettings(input: {
    connectionId: string;
    settings: SalesforceSettings;
}): Promise<{ settings: SalesforceSettings }> {
    return await Request<{ settings: SalesforceSettings }>({
        method: "PUT",
        url: `${base(input.connectionId)}/settings`,
        data: input.settings,
        authorization: true,
    });
}

export async function getSalesforceMetadata(id: string): Promise<SalesforceMetadata> {
    return await Request<SalesforceMetadata>({
        method: "GET",
        url: `${base(id)}/metadata`,
        authorization: true,
    });
}

export async function searchSalesforceUsers(id: string, q: string): Promise<SalesforceUser[]> {
    const body = await Request<{ data: SalesforceUser[] }>({
        method: "GET",
        url: `${base(id)}/users`,
        params: { q },
        authorization: true,
    });
    return body?.data ?? [];
}

export async function listSalesforceListViews(id: string, object: "Lead" | "Contact"): Promise<SalesforceListView[]> {
    const body = await Request<{ data: SalesforceListView[] }>({
        method: "GET",
        url: `${base(id)}/list-views`,
        params: { object },
        authorization: true,
    });
    return body?.data ?? [];
}

export async function searchSalesforceCampaigns(id: string, q: string): Promise<SalesforceCampaign[]> {
    const body = await Request<{ data: SalesforceCampaign[] }>({
        method: "GET",
        url: `${base(id)}/campaigns`,
        params: { q },
        authorization: true,
    });
    return body?.data ?? [];
}

export async function previewSalesforceImport(input: {
    connectionId: string;
    source_kind: SalesforceSourceKind;
    object: SalesforceImportObject;
    source_id: string;
}): Promise<SalesforceImportPreview> {
    const { connectionId, ...body } = input;
    return await Request<SalesforceImportPreview>({
        method: "POST",
        url: `${base(connectionId)}/import/preview`,
        data: body,
        authorization: true,
    });
}

export async function listSalesforceImportSources(id: string): Promise<SalesforceImportSource[]> {
    const body = await Request<{ data: SalesforceImportSource[] }>({
        method: "GET",
        url: `${base(id)}/import-sources`,
        authorization: true,
    });
    return body?.data ?? [];
}

export async function createSalesforceImportSource(input: {
    connectionId: string;
    body: CreateSalesforceImportSourceInput;
}): Promise<SalesforceImportSource> {
    return await Request<SalesforceImportSource>({
        method: "POST",
        url: `${base(input.connectionId)}/import-sources`,
        data: input.body,
        authorization: true,
    });
}

export async function updateSalesforceImportSource(input: {
    connectionId: string;
    sourceId: string;
    body: UpdateSalesforceImportSourceInput;
}): Promise<SalesforceImportSource> {
    return await Request<SalesforceImportSource>({
        method: "PATCH",
        url: `${base(input.connectionId)}/import-sources/${input.sourceId}`,
        data: input.body,
        authorization: true,
    });
}

export async function runSalesforceImportSource(input: {
    connectionId: string;
    sourceId: string;
}): Promise<SalesforceImportSource> {
    return await Request<SalesforceImportSource>({
        method: "POST",
        url: `${base(input.connectionId)}/import-sources/${input.sourceId}/run`,
        data: {},
        authorization: true,
    });
}

export async function deleteSalesforceImportSource(input: { connectionId: string; sourceId: string }): Promise<void> {
    await Request<void>({
        method: "DELETE",
        url: `${base(input.connectionId)}/import-sources/${input.sourceId}`,
        authorization: true,
    });
}

export async function listSalesforceActivity(input: {
    connectionId: string;
    status?: SalesforceActivityStatus | "";
    contactId?: string;
    cursor?: string | null;
    limit?: number;
}): Promise<SalesforceActivityPage> {
    const params: Record<string, string | number> = {};
    if (input.status) params.status = input.status;
    if (input.contactId) params.contact_id = input.contactId;
    if (input.cursor) params.cursor = input.cursor;
    if (input.limit) params.limit = input.limit;
    return await Request<SalesforceActivityPage>({
        method: "GET",
        url: `${base(input.connectionId)}/activity`,
        params,
        authorization: true,
    });
}

export async function retrySalesforceActivity(input: {
    connectionId: string;
    ids?: string[];
}): Promise<{ requeued: number }> {
    return await Request<{ requeued: number }>({
        method: "POST",
        url: `${base(input.connectionId)}/activity/retry`,
        data: input.ids && input.ids.length > 0 ? { ids: input.ids } : {},
        authorization: true,
    });
}

export async function salesforceSyncNow(id: string): Promise<{ ok: boolean }> {
    return await Request<{ ok: boolean }>({
        method: "POST",
        url: `${base(id)}/sync-now`,
        data: {},
        authorization: true,
    });
}

export async function getContactSalesforce(contactId: string): Promise<ContactSalesforcePanel> {
    return await Request<ContactSalesforcePanel>({
        method: "GET",
        url: `/contacts/${contactId}/salesforce`,
        authorization: true,
    });
}

export async function syncContactSalesforce(input: {
    contactId: string;
    connection_id?: string;
    create_as?: "lead" | "contact";
}): Promise<ContactSalesforcePanel> {
    const { contactId, ...body } = input;
    return await Request<ContactSalesforcePanel>({
        method: "POST",
        url: `/contacts/${contactId}/salesforce/sync`,
        data: body,
        authorization: true,
    });
}

export async function unlinkContactSalesforce(input: { contactId: string; linkId: string }): Promise<void> {
    await Request<void>({
        method: "DELETE",
        url: `/contacts/${input.contactId}/salesforce/links/${input.linkId}`,
        authorization: true,
    });
}
