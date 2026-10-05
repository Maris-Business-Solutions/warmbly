import type { InfiniteData, QueryClient, QueryKey } from "@tanstack/react-query";
import type Deal from "@/lib/api/models/app/crm/Deal";
import type { DealWrite } from "@/lib/api/models/app/crm/Deal";
import type DealsSummary from "@/lib/api/models/app/crm/DealsSummary";
import type DealsSearchResult from "@/lib/api/models/app/crm/DealsSearchResult";
import type Pipeline from "@/lib/api/models/app/crm/Pipeline";
import type SearchDeals from "@/lib/api/models/app/crm/SearchDeals";
import { findEntity, removeEntities } from "@/lib/api/hooks/optimistic";

export const dealMutationKey = ["crm", "deals"];

// Every cache a deal row shows in: the deal lists and boards, and its contact's deal panel.
export function dealKeys(contactId?: string): QueryKey[] {
    return contactId ? [["crm", "deals"], ["contacts", contactId, "deals"]] : [["crm", "deals"]];
}

// Looks in the deal lists first, then only the contacts' deal panels, never every contact query.
export function cachedDeal(queryClient: QueryClient, id: string): Deal | undefined {
    const listed = findEntity<Deal>(queryClient, [["crm", "deals"]], id);
    if (listed) return listed;
    const panels = queryClient
        .getQueryCache()
        .findAll({ predicate: (q) => q.queryKey[0] === "contacts" && q.queryKey[2] === "deals" })
        .map((q) => q.queryKey);
    return panels.length ? findEntity<Deal>(queryClient, panels, id) : undefined;
}

// A write as the row will read once the server has it, with the joined stage looked up from the pipelines.
export function applyDealWrite(queryClient: QueryClient, row: Deal, data: DealWrite): Deal {
    const { expected_close_date, ...rest } = data;
    const next: Deal = { ...row, ...rest, id: row.id };
    if ("expected_close_date" in data) {
        next.expected_close_date = expected_close_date ? new Date(expected_close_date) : undefined;
    }
    if (data.stage_id && data.stage_id !== row.stage_id) {
        const pipelines = queryClient.getQueryData<Pipeline[]>(["crm", "pipelines", "list"]);
        const stage = pipelines?.flatMap((p) => p.stages ?? []).find((s) => s.id === data.stage_id);
        next.stage = stage
            ? { id: stage.id, name: stage.name, color: stage.color, position: stage.position }
            : undefined;
        next.updated_at = new Date();
    }
    return next;
}

type SearchKey = [string, string, string, SearchDeals, number];

const sameBut = (a: SearchDeals, b: SearchDeals) =>
    JSON.stringify({ ...a, stage_ids: [] }) === JSON.stringify({ ...b, stage_ids: [] });

// Moves a deal between stage-scoped searches (the board's columns): out of a
// column that no longer matches, and onto the top of the matching column of
// the same board when that column is newest-updated first.
export function moveAcrossStages(queryClient: QueryClient, before: Deal, after: Deal) {
    const searches = queryClient.getQueriesData<InfiniteData<DealsSearchResult>>({ queryKey: ["crm", "deals", "search"] });
    const boards: SearchDeals[] = [];
    for (const [key, data] of searches) {
        const filters = (key as SearchKey)[3];
        if (!data || !filters?.stage_ids?.length || filters.stage_ids.includes(after.stage_id)) continue;
        const next = removeEntities([before.id])(data, key);
        if (next === data) continue;
        boards.push(filters);
        queryClient.setQueryData(key, next);
    }
    for (const [key, data] of searches) {
        const filters = (key as SearchKey)[3];
        if (!data?.pages?.length || !filters?.stage_ids?.includes(after.stage_id)) continue;
        if (filters.sort_by !== "updated_at" || filters.reverse) continue;
        if (!boards.some((b) => sameBut(b, filters))) continue;
        if (data.pages.some((p) => (p.data ?? []).some((d) => d?.id === before.id))) continue;
        const [first, ...rest] = data.pages;
        const pagination = { ...first.pagination, total: (first.pagination?.total ?? 0) + 1 };
        queryClient.setQueryData(key, {
            ...data,
            pages: [
                { ...first, data: [after, ...(first.data ?? [])], pagination },
                ...rest.map((p) => ({ ...p, pagination: { ...p.pagination, total: pagination.total } })),
            ],
        });
    }
    // The per-stage board headers: one fewer in the old stage, one more in the new.
    const value = before.value ?? 0;
    queryClient.setQueriesData<DealsSummary>({ queryKey: ["crm", "deals", "summary"] }, (old) => {
        if (!old?.stages?.some((s) => s.stage_id === before.stage_id)) return old;
        const stages = old.stages.map((s) =>
            s.stage_id === before.stage_id
                ? { ...s, count: Math.max(0, s.count - 1), value: s.value - value }
                : s.stage_id === after.stage_id
                  ? { ...s, count: s.count + 1, value: s.value + value }
                  : s,
        );
        return { ...old, stages };
    });
}
