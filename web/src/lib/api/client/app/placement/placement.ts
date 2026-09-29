import type {
    CreatePlacementTestRequest,
    PlacementBatch,
    PlacementBatchDetail,
    PlacementBatchList,
    PlacementBatchPreview,
    PlacementBatchRequest,
    PlacementBatchSenderList,
    PlacementBatchSenderSort,
    PlacementBatchSenderStatus,
    PlacementCoverage,
    PlacementMonitor,
    PlacementMonitorInput,
    PlacementOverview,
    PlacementTest,
    PlacementTestDetail,
    PlacementTestList,
    PlacementWorkspaceSeed,
} from "@/lib/api/models/app/placement/Placement";
import Request from "../../Request";

export async function getPlacementOverview(): Promise<PlacementOverview> {
    const res = await Request<{ data: PlacementOverview }>({
        method: "GET",
        url: "/placement/overview",
        authorization: true,
    });
    return res.data;
}

export async function listPlacementTests(
    cursor: string | null,
    limit: number,
    campaignId?: string | null,
): Promise<PlacementTestList> {
    const params = new URLSearchParams();
    params.set("limit", String(limit));
    if (cursor) params.set("cursor", cursor);
    if (campaignId) params.set("campaign_id", campaignId);
    return await Request<PlacementTestList>({
        method: "GET",
        url: `/placement/tests?${params.toString()}`,
        authorization: true,
    });
}

export async function getPlacementTest(id: string): Promise<PlacementTestDetail> {
    const res = await Request<{ data: PlacementTestDetail }>({
        method: "GET",
        url: `/placement/tests/${id}`,
        authorization: true,
    });
    return res.data;
}

// One test, or two sharing a compare_group_id for a tracking comparison. The
// key makes a retried submit land on the tests the first attempt started.
export async function createPlacementTest(
    body: CreatePlacementTestRequest,
    idempotencyKey?: string,
): Promise<PlacementTest[]> {
    const res = await Request<{ data: PlacementTest[] | null }>({
        method: "POST",
        url: "/placement/tests",
        data: body,
        headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
        authorization: true,
    });
    return res.data ?? [];
}

// Stops the copies not sent yet; the sent ones keep being classified.
export async function cancelPlacementTest(id: string): Promise<PlacementTest> {
    const res = await Request<{ data: PlacementTest }>({
        method: "POST",
        url: `/placement/tests/${id}/cancel`,
        authorization: true,
    });
    return res.data;
}

export async function listPlacementSeeds(): Promise<PlacementWorkspaceSeed[]> {
    const res = await Request<{ data: PlacementWorkspaceSeed[] | null }>({
        method: "GET",
        url: "/placement/seeds",
        authorization: true,
    });
    return res.data ?? [];
}

export async function setPlacementSeed(emailAccountId: string, seed: boolean): Promise<PlacementWorkspaceSeed> {
    const res = await Request<{ data: PlacementWorkspaceSeed }>({
        method: "PUT",
        url: `/placement/seeds/${emailAccountId}`,
        data: { seed },
        authorization: true,
    });
    return res.data;
}

export async function getPlacementMonitor(campaignId: string): Promise<PlacementMonitor | null> {
    const res = await Request<{ data: PlacementMonitor | null }>({
        method: "GET",
        url: `/campaigns/${campaignId}/placement-monitor`,
        authorization: true,
    });
    return res.data ?? null;
}

// A full-state write, so a retry lands on the same monitor.
export async function putPlacementMonitor(campaignId: string, input: PlacementMonitorInput): Promise<PlacementMonitor> {
    const res = await Request<{ data: PlacementMonitor }>({
        method: "PUT",
        url: `/campaigns/${campaignId}/placement-monitor`,
        data: input,
        authorization: true,
    });
    return res.data;
}

export async function deletePlacementMonitor(campaignId: string): Promise<void> {
    await Request<void>({
        method: "DELETE",
        url: `/campaigns/${campaignId}/placement-monitor`,
        authorization: true,
    });
}

// Batches: one placement test run from many senders.

export async function previewPlacementBatch(body: PlacementBatchRequest): Promise<PlacementBatchPreview> {
    const res = await Request<{ data: PlacementBatchPreview }>({
        method: "POST",
        url: "/placement/batches/preview",
        data: body,
        authorization: true,
    });
    return res.data;
}

// The key makes a retried submit land on the batch the first attempt queued.
export async function createPlacementBatch(body: PlacementBatchRequest, idempotencyKey?: string): Promise<PlacementBatch> {
    const res = await Request<{ data: PlacementBatch }>({
        method: "POST",
        url: "/placement/batches",
        data: body,
        headers: idempotencyKey ? { "Idempotency-Key": idempotencyKey } : undefined,
        authorization: true,
    });
    return res.data;
}

export async function listPlacementBatches(cursor: string | null, limit: number): Promise<PlacementBatchList> {
    const params = new URLSearchParams();
    params.set("limit", String(limit));
    if (cursor) params.set("cursor", cursor);
    return await Request<PlacementBatchList>({
        method: "GET",
        url: `/placement/batches?${params.toString()}`,
        authorization: true,
    });
}

export async function getPlacementBatch(id: string): Promise<PlacementBatchDetail> {
    const res = await Request<{ data: PlacementBatchDetail }>({
        method: "GET",
        url: `/placement/batches/${id}`,
        authorization: true,
    });
    return res.data;
}

export async function listPlacementBatchSenders(
    id: string,
    opts: { cursor: string | null; limit: number; sort?: PlacementBatchSenderSort; status?: PlacementBatchSenderStatus | ""; q?: string },
): Promise<PlacementBatchSenderList> {
    const params = new URLSearchParams();
    params.set("limit", String(opts.limit));
    if (opts.cursor) params.set("cursor", opts.cursor);
    if (opts.sort) params.set("sort", opts.sort);
    if (opts.status) params.set("status", opts.status);
    if (opts.q) params.set("q", opts.q);
    return await Request<PlacementBatchSenderList>({
        method: "GET",
        url: `/placement/batches/${id}/senders?${params.toString()}`,
        authorization: true,
    });
}

// Stops the batch; copies already sent keep being classified.
export async function cancelPlacementBatch(id: string): Promise<PlacementBatch> {
    const res = await Request<{ data: PlacementBatch }>({
        method: "POST",
        url: `/placement/batches/${id}/cancel`,
        authorization: true,
    });
    return res.data;
}

export async function getPlacementCoverage(): Promise<PlacementCoverage> {
    const res = await Request<{ data: PlacementCoverage }>({
        method: "GET",
        url: "/placement/coverage",
        authorization: true,
    });
    return res.data;
}
