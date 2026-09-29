import { useContext } from "react";
import { SocketContext } from "@/hooks/context/socket";
import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import {
    cancelPlacementBatch,
    cancelPlacementTest,
    createPlacementBatch,
    createPlacementTest,
    deletePlacementMonitor,
    getPlacementMonitor,
    getPlacementBatch,
    getPlacementCoverage,
    getPlacementOverview,
    getPlacementTest,
    listPlacementBatchSenders,
    listPlacementBatches,
    listPlacementSeeds,
    listPlacementTests,
    previewPlacementBatch,
    putPlacementMonitor,
    setPlacementSeed,
} from "@/lib/api/client/app/placement/placement";
import type {
    CreatePlacementTestRequest,
    PlacementBatchList,
    PlacementBatchRequest,
    PlacementBatchSenderList,
    PlacementBatchSenderSort,
    PlacementBatchSenderStatus,
    PlacementMonitorInput,
    PlacementTestList,
} from "@/lib/api/models/app/placement/Placement";

// Everything lives under ["placement"], which PLACEMENT_TEST_UPDATED and the
// placement_test / placement_monitor audit spine invalidate, so these views
// stay live with no polling.
export const PLACEMENT_KEY = ["placement"] as const;

export function usePlacementOverview(enabled = true) {
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "overview"],
        queryFn: getPlacementOverview,
        enabled,
    });
}

export function usePlacementTests(campaignId: string | null = null, limit = 25) {
    const query = useInfiniteQuery<
        PlacementTestList,
        Error,
        InfiniteData<PlacementTestList, string | null>,
        (string | number | null)[],
        string | null
    >({
        queryKey: [...PLACEMENT_KEY, "tests", campaignId, limit],
        queryFn: ({ pageParam }) => listPlacementTests(pageParam, limit, campaignId),
        initialPageParam: null,
        getNextPageParam: (last) => (last.pagination.has_more ? last.pagination.next_cursor : undefined),
        placeholderData: keepPreviousData,
    });
    const tests = query.data?.pages.flatMap((p) => p.data ?? []) ?? [];
    const total = query.data?.pages[0]?.pagination.total ?? null;
    return { ...query, tests, total };
}

export function usePlacementTest(id: string) {
    // Realtime drives a running test; only a dropped socket falls back to a slow poll.
    const socketUp = useContext(SocketContext)?.isConnected ?? true;
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "test", id],
        queryFn: () => getPlacementTest(id),
        enabled: !!id,
        refetchInterval: (query) => (!socketUp && query.state.data?.status === "running" ? 15_000 : false),
    });
}

export function usePlacementSeeds(enabled = true) {
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "seeds"],
        queryFn: listPlacementSeeds,
        enabled,
    });
}

export function useCreatePlacementTest() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: ({ body, idempotencyKey }: { body: CreatePlacementTestRequest; idempotencyKey?: string }) =>
            createPlacementTest(body, idempotencyKey),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: PLACEMENT_KEY });
            // Every copy is charged to the sender's daily limit.
            qc.invalidateQueries({ queryKey: ["emails", "list"] });
        },
    });
}

export function useCancelPlacementTest() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (id: string) => cancelPlacementTest(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: PLACEMENT_KEY }),
    });
}

export function useSetPlacementSeed() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: ({ emailAccountId, seed }: { emailAccountId: string; seed: boolean }) =>
            setPlacementSeed(emailAccountId, seed),
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: PLACEMENT_KEY });
            // Marking a seed turns its warmup off.
            qc.invalidateQueries({ queryKey: ["emails"] });
        },
    });
}

export function usePlacementMonitor(campaignId: string) {
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "monitor", campaignId],
        queryFn: () => getPlacementMonitor(campaignId),
        enabled: !!campaignId,
    });
}

export function usePutPlacementMonitor(campaignId: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (input: PlacementMonitorInput) => putPlacementMonitor(campaignId, input),
        onSuccess: (m) => {
            qc.setQueryData([...PLACEMENT_KEY, "monitor", campaignId], m);
        },
    });
}

export function useDeletePlacementMonitor(campaignId: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: () => deletePlacementMonitor(campaignId),
        onSuccess: () => {
            qc.setQueryData([...PLACEMENT_KEY, "monitor", campaignId], null);
        },
    });
}

export function usePlacementBatches(limit = 10, enabled = true) {
    const query = useInfiniteQuery<
        PlacementBatchList,
        Error,
        InfiniteData<PlacementBatchList, string | null>,
        (string | number)[],
        string | null
    >({
        queryKey: [...PLACEMENT_KEY, "batches", limit],
        queryFn: ({ pageParam }) => listPlacementBatches(pageParam, limit),
        initialPageParam: null,
        getNextPageParam: (last) => (last.pagination.has_more ? last.pagination.next_cursor : undefined),
        placeholderData: keepPreviousData,
        enabled,
    });
    const batches = query.data?.pages.flatMap((p) => p.data ?? []) ?? [];
    const total = query.data?.pages[0]?.pagination.total ?? null;
    return { ...query, batches, total };
}

export function usePlacementBatch(id: string) {
    // Child test events drive a running batch; a dropped socket falls back to a slow poll.
    const socketUp = useContext(SocketContext)?.isConnected ?? true;
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "batch", id],
        queryFn: () => getPlacementBatch(id),
        enabled: !!id,
        refetchInterval: (query) => {
            const status = query.state.data?.status;
            return !socketUp && (status === "running" || status === "queued") ? 30_000 : false;
        },
    });
}

export function usePlacementBatchSenders(
    id: string,
    opts: { sort: PlacementBatchSenderSort; status: PlacementBatchSenderStatus | ""; q: string; limit?: number },
) {
    const limit = opts.limit ?? 50;
    const query = useInfiniteQuery<
        PlacementBatchSenderList,
        Error,
        InfiniteData<PlacementBatchSenderList, string | null>,
        (string | number)[],
        string | null
    >({
        queryKey: [...PLACEMENT_KEY, "batch", id, "senders", opts.sort, opts.status, opts.q, limit],
        queryFn: ({ pageParam }) =>
            listPlacementBatchSenders(id, { cursor: pageParam, limit, sort: opts.sort, status: opts.status, q: opts.q }),
        initialPageParam: null,
        getNextPageParam: (last) => (last.pagination.has_more ? last.pagination.next_cursor : undefined),
        placeholderData: keepPreviousData,
        enabled: !!id,
    });
    const senders = query.data?.pages.flatMap((p) => p.data ?? []) ?? [];
    const total = query.data?.pages[0]?.pagination.total ?? null;
    return { ...query, senders, total };
}

// A preview for the current selection; the caller debounces the body.
export function usePlacementBatchPreview(body: PlacementBatchRequest | null) {
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "batch-preview", body],
        queryFn: () => previewPlacementBatch(body as PlacementBatchRequest),
        enabled: body != null,
        placeholderData: keepPreviousData,
        retry: false,
    });
}

export function useCreatePlacementBatch() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: ({ body, idempotencyKey }: { body: PlacementBatchRequest; idempotencyKey?: string }) =>
            createPlacementBatch(body, idempotencyKey),
        onSuccess: () => qc.invalidateQueries({ queryKey: PLACEMENT_KEY }),
    });
}

export function useCancelPlacementBatch() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (id: string) => cancelPlacementBatch(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: PLACEMENT_KEY }),
    });
}

export function usePlacementCoverage(enabled = true) {
    return useQuery({
        queryKey: [...PLACEMENT_KEY, "coverage"],
        queryFn: getPlacementCoverage,
        enabled,
    });
}
