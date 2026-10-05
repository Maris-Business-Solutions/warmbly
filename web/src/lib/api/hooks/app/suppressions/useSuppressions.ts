import { infiniteQueryOptions, keepPreviousData, useInfiniteQuery, useMutation, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import { DEFAULT_PAGINATION_LIMIT } from "@/lib/information";
import { addSuppressions, listSuppressions, removeSuppression } from "@/lib/api/client/app/suppressions/suppressions";
import type { AddSuppressionsRequest, SuppressionListResult } from "@/lib/api/models/app/suppressions/Suppression";

export const SUPPRESSIONS_KEY = ["suppressions"];

export const suppressionsListQuery = (q: string, limit = DEFAULT_PAGINATION_LIMIT) =>
    infiniteQueryOptions<
        SuppressionListResult,
        Error,
        InfiniteData<SuppressionListResult, string | null>,
        (string | number)[],
        string | null
    >({
        queryKey: [...SUPPRESSIONS_KEY, "list", q, limit],
        queryFn: ({ pageParam }) => listSuppressions(q, pageParam, limit),
        initialPageParam: null,
        getNextPageParam: (last) => (last.pagination.has_more ? last.pagination.next_cursor : undefined),
        staleTime: 30_000,
    });

export function useSuppressions(q: string, limit = DEFAULT_PAGINATION_LIMIT) {
    const query = useInfiniteQuery({
        ...suppressionsListQuery(q, limit),
        placeholderData: keepPreviousData,
    });
    const entries = query.data?.pages.flatMap((p) => p.data ?? []) ?? [];
    return { ...query, entries };
}

// Both mutations touch who a campaign can reach, so the contact views and
// the deliverability numbers refresh along with the list.
function invalidateAfterChange(qc: ReturnType<typeof useQueryClient>) {
    qc.invalidateQueries({ queryKey: SUPPRESSIONS_KEY });
    qc.invalidateQueries({ queryKey: ["contacts"] });
    qc.invalidateQueries({ queryKey: ["analytics"] });
}

export function useAddSuppressions() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (data: AddSuppressionsRequest) => addSuppressions(data),
        onSuccess: () => invalidateAfterChange(qc),
    });
}

export function useRemoveSuppression() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (id: string) => removeSuppression(id),
        onSuccess: () => invalidateAfterChange(qc),
    });
}
