import {
    keepPreviousData,
    useInfiniteQuery,
    useMutation,
    useQuery,
    useQueryClient,
    type InfiniteData,
} from "@tanstack/react-query";
import {
    createSalesforceImportSource,
    deleteSalesforceImportSource,
    getContactSalesforce,
    getSalesforceMetadata,
    getSalesforceOverview,
    getSalesforceSettings,
    listSalesforceActivity,
    listSalesforceImportSources,
    listSalesforceListViews,
    previewSalesforceImport,
    retrySalesforceActivity,
    runSalesforceImportSource,
    salesforceSyncNow,
    searchSalesforceCampaigns,
    searchSalesforceUsers,
    syncContactSalesforce,
    unlinkContactSalesforce,
    updateSalesforceImportSource,
    updateSalesforceSettings,
} from "@/lib/api/client/app/integrations/salesforce";
import type {
    ContactSalesforcePanel,
    SalesforceActivityPage,
    SalesforceActivityStatus,
} from "@/lib/api/models/app/integrations/Salesforce";

// Everything the realtime spine refreshes for entity_type "integration".
export const SALESFORCE_KEY = ["integrations", "salesforce"] as const;
// Lookups that each cost Salesforce API calls live outside that prefix, so a
// teammate's settings save does not re-describe the org for everyone.
const LOOKUP_KEY = "salesforce-lookup";

const sfKey = (id: string, ...rest: unknown[]) => [...SALESFORCE_KEY, id, ...rest];

export function contactSalesforceKey(contactId: string) {
    return ["contacts", contactId, "salesforce"];
}

export function useSalesforceOverview(id: string) {
    return useQuery({
        queryKey: sfKey(id, "overview"),
        queryFn: () => getSalesforceOverview(id),
        enabled: !!id,
        staleTime: 15_000,
    });
}

// Runs the permission check pass and folds the result into the overview cache.
export function useSalesforcePermissionCheck(id: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: () => getSalesforceOverview(id, true),
        onSuccess: (data) => qc.setQueryData(sfKey(id, "overview"), data),
    });
}

export function useSalesforceSettings(id: string) {
    return useQuery({
        queryKey: sfKey(id, "settings"),
        queryFn: () => getSalesforceSettings(id),
        enabled: !!id,
        staleTime: 30_000,
    });
}

export function useUpdateSalesforceSettings(id: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: updateSalesforceSettings,
        onSuccess: (res) => {
            qc.setQueryData(sfKey(id, "settings"), (prev: unknown) =>
                prev && typeof prev === "object" ? { ...(prev as object), settings: res.settings } : prev,
            );
            qc.invalidateQueries({ queryKey: sfKey(id, "overview") });
        },
    });
}

export function useSalesforceMetadata(id: string) {
    return useQuery({
        queryKey: [LOOKUP_KEY, id, "metadata"],
        queryFn: () => getSalesforceMetadata(id),
        enabled: !!id,
        staleTime: 10 * 60 * 1000,
    });
}

export function useSalesforceUsers(id: string, q: string, enabled = true) {
    return useQuery({
        queryKey: [LOOKUP_KEY, id, "users", q],
        queryFn: () => searchSalesforceUsers(id, q),
        enabled: !!id && enabled,
        staleTime: 60_000,
        placeholderData: keepPreviousData,
    });
}

export function useSalesforceListViews(id: string, object: "Lead" | "Contact", enabled = true) {
    return useQuery({
        queryKey: [LOOKUP_KEY, id, "list-views", object],
        queryFn: () => listSalesforceListViews(id, object),
        enabled: !!id && enabled,
        staleTime: 5 * 60 * 1000,
    });
}

export function useSalesforceCampaigns(id: string, q: string, enabled = true) {
    return useQuery({
        queryKey: [LOOKUP_KEY, id, "campaigns", q],
        queryFn: () => searchSalesforceCampaigns(id, q),
        enabled: !!id && enabled,
        staleTime: 60_000,
        placeholderData: keepPreviousData,
    });
}

export function useSalesforceImportPreview() {
    return useMutation({ mutationFn: previewSalesforceImport });
}

// No realtime event marks a run finishing, so the list polls only while one runs.
export function useSalesforceImportSources(id: string) {
    return useQuery({
        queryKey: sfKey(id, "import-sources"),
        queryFn: () => listSalesforceImportSources(id),
        enabled: !!id,
        staleTime: 10_000,
        refetchInterval: (q) => ((q.state.data ?? []).some((s) => s.status === "running") ? 3000 : false),
    });
}

function useInvalidateSources(id: string) {
    const qc = useQueryClient();
    return () => {
        qc.invalidateQueries({ queryKey: sfKey(id, "import-sources") });
        qc.invalidateQueries({ queryKey: sfKey(id, "overview") });
    };
}

export function useCreateSalesforceImportSource(id: string) {
    const refresh = useInvalidateSources(id);
    const qc = useQueryClient();
    return useMutation({
        mutationFn: createSalesforceImportSource,
        onSuccess: () => {
            refresh();
            qc.invalidateQueries({ queryKey: ["contacts"] });
        },
    });
}

export function useUpdateSalesforceImportSource(id: string) {
    const refresh = useInvalidateSources(id);
    return useMutation({ mutationFn: updateSalesforceImportSource, onSuccess: refresh });
}

export function useRunSalesforceImportSource(id: string) {
    const refresh = useInvalidateSources(id);
    return useMutation({ mutationFn: runSalesforceImportSource, onSuccess: refresh });
}

export function useDeleteSalesforceImportSource(id: string) {
    const refresh = useInvalidateSources(id);
    return useMutation({ mutationFn: deleteSalesforceImportSource, onSuccess: refresh });
}

export function useSalesforceActivity(id: string, status: SalesforceActivityStatus | "", limit = 50) {
    const query = useInfiniteQuery<
        SalesforceActivityPage,
        Error,
        InfiniteData<SalesforceActivityPage, string | null>,
        unknown[],
        string | null
    >({
        queryKey: sfKey(id, "activity", status, limit),
        queryFn: ({ pageParam }) => listSalesforceActivity({ connectionId: id, status, cursor: pageParam, limit }),
        initialPageParam: null,
        getNextPageParam: (last) => (last.pagination?.has_more ? (last.pagination.next_cursor ?? undefined) : undefined),
        placeholderData: keepPreviousData,
        enabled: !!id,
        staleTime: 15_000,
    });
    const rows = query.data?.pages.flatMap((p) => p.data ?? []) ?? [];
    return { ...query, rows };
}

export function useRetrySalesforceActivity(id: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: retrySalesforceActivity,
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: sfKey(id, "activity") });
            qc.invalidateQueries({ queryKey: sfKey(id, "overview") });
        },
    });
}

export function useSalesforceSyncNow(id: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: () => salesforceSyncNow(id),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: [...SALESFORCE_KEY, id] });
            qc.invalidateQueries({ queryKey: ["integrations", "connection", id] });
        },
    });
}

// The contact drawer's Salesforce panel. Keyed under ["contacts", id] so a
// contact change refreshes it; each fetch reads Salesforce live, so it is
// cached for a minute rather than refetched on every focus.
export function useContactSalesforce(contactId: string | undefined, enabled = true) {
    return useQuery({
        queryKey: contactSalesforceKey(contactId ?? ""),
        queryFn: () => getContactSalesforce(contactId as string),
        enabled: !!contactId && enabled,
        staleTime: 60_000,
        refetchOnWindowFocus: false,
        retry: false,
    });
}

export function useSyncContactSalesforce(contactId: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (input: { connection_id?: string; create_as?: "lead" | "contact" }) =>
            syncContactSalesforce({ contactId, ...input }),
        onSuccess: (panel: ContactSalesforcePanel) => {
            qc.setQueryData(contactSalesforceKey(contactId), panel);
            qc.invalidateQueries({ queryKey: SALESFORCE_KEY });
        },
    });
}

export function useUnlinkContactSalesforce(contactId: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (linkId: string) => unlinkContactSalesforce({ contactId, linkId }),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: contactSalesforceKey(contactId) });
            qc.invalidateQueries({ queryKey: SALESFORCE_KEY });
        },
    });
}
