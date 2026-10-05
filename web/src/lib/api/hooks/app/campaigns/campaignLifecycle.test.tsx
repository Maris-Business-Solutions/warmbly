// Issue #185: delete and duplicate from the campaigns UI. The list is an
// infinite query with a 5 minute staleTime, so a deleted campaign has to leave
// the cached pages immediately (before the server answers, and back if it
// refuses) and a duplicate has to appear in them, or the page keeps showing
// the old set until something else refetches it.

import React from "react";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { QueryClient, QueryClientProvider, type InfiniteData } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type Campaign from "@/lib/api/models/app/campaigns/Campaign";
import type GetCampaigns from "@/lib/api/models/app/campaigns/GetCampaigns";

const getCampaigns = vi.fn();
const deleteCampaign = vi.fn();
const duplicateCampaign = vi.fn();

vi.mock("@/lib/api/client/app/campaigns/getCampaigns", () => ({
    default: (...args: unknown[]) => getCampaigns(...args),
}));
vi.mock("@/lib/api/client/app/campaigns/deleteCampaign", () => ({
    default: (...args: unknown[]) => deleteCampaign(...args),
}));
vi.mock("@/lib/api/client/app/campaigns/duplicateCampaign", () => ({
    default: (...args: unknown[]) => duplicateCampaign(...args),
}));

const useCampaigns = (await import("./useCampaigns")).default;
const useDeleteCampaign = (await import("./useDeleteCampaign")).default;
const useDuplicateCampaign = (await import("./useDuplicateCampaign")).default;

function campaign(id: string, name: string, status = "active"): Campaign {
    return { id, name, status, description: "" } as unknown as Campaign;
}

function page(data: Campaign[]): GetCampaigns {
    return { data, pagination: { total: data.length, next_cursor: null, has_more: false } } as GetCampaigns;
}

// The ids in the cached list pages right now, before any refetch settles.
function cachedIds(client: QueryClient): string[] {
    const [entry] = client.getQueriesData<InfiniteData<GetCampaigns>>({ queryKey: ["campaigns", "list", "", ""] });
    return entry?.[1]?.pages.flatMap((p) => p.data.map((c) => c.id)) ?? [];
}

function wrapper(client: QueryClient) {
    return function Wrapper({ children }: { children: React.ReactNode }) {
        return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    };
}

const A = campaign("camp-a", "Agency partnerships");
const B = campaign("camp-b", "RevOps outreach", "draft");

describe("campaign delete and duplicate keep the campaigns list current", () => {
    let client: QueryClient;
    let server: Campaign[];

    beforeEach(() => {
        vi.clearAllMocks();
        client = new QueryClient({
            defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        });
        server = [A, B];
        getCampaigns.mockImplementation(async () => page([...server]));
        deleteCampaign.mockImplementation(async (id: string) => {
            server = server.filter((c) => c.id !== id);
        });
        duplicateCampaign.mockImplementation(async (id: string) => {
            const source = server.find((c) => c.id === id)!;
            const copy = campaign(`${id}-copy`, `${source.name} (copy)`, "draft");
            server = [copy, ...server];
            return copy;
        });
    });

    it("removes a deleted campaign from every cached list page without waiting for a refetch", async () => {
        const list = renderHook(() => useCampaigns({ query: "", folder: "" }), { wrapper: wrapper(client) });
        await waitFor(() => expect(list.result.current.isSuccess).toBe(true));
        expect(list.result.current.campaigns.map((c) => c.id)).toEqual(["camp-a", "camp-b"]);
        client.setQueryData(["campaigns", "camp-a"], A);

        const del = renderHook(() => useDeleteCampaign(), { wrapper: wrapper(client) });
        await del.result.current.mutateAsync("camp-a");

        expect(deleteCampaign).toHaveBeenCalledWith("camp-a");
        // The cache was patched, not merely invalidated: the row is gone from
        // the stored pages before any refetch has answered.
        expect(cachedIds(client)).toEqual(["camp-b"]);
        expect(client.getQueryData(["campaigns", "camp-a"])).toBeUndefined();

        await waitFor(() => expect(getCampaigns).toHaveBeenCalledTimes(2));
        await waitFor(() => expect(list.result.current.campaigns.map((c) => c.id)).toEqual(["camp-b"]));
    });

    it("places the duplicate at the top of the list and primes its detail cache", async () => {
        const list = renderHook(() => useCampaigns({ query: "", folder: "" }), { wrapper: wrapper(client) });
        await waitFor(() => expect(list.result.current.isSuccess).toBe(true));
        // A filtered list may not contain the copy, so it is refetched, never patched.
        client.setQueryData(["campaigns", "list", "webinar", "", 20], {
            pages: [page([B])],
            pageParams: [null],
        });

        const dup = renderHook(() => useDuplicateCampaign(), { wrapper: wrapper(client) });
        const copy = await dup.result.current.mutateAsync({ id: "camp-a" });

        expect(duplicateCampaign).toHaveBeenCalledWith("camp-a", undefined);
        expect(copy.name).toBe("Agency partnerships (copy)");
        expect(copy.status).toBe("draft");
        expect(cachedIds(client)).toEqual(["camp-a-copy", "camp-a", "camp-b"]);
        const filtered = client.getQueryData<InfiniteData<GetCampaigns>>(["campaigns", "list", "webinar", "", 20]);
        expect(filtered?.pages[0].data.map((c) => c.id)).toEqual(["camp-b"]);
        // Navigating straight to the copy must not flash a skeleton.
        expect(client.getQueryData(["campaigns", "camp-a-copy"])).toEqual(copy);

        await waitFor(() => expect(getCampaigns).toHaveBeenCalledTimes(2));
        await waitFor(() =>
            expect(list.result.current.campaigns.map((c) => c.id)).toEqual(["camp-a-copy", "camp-a", "camp-b"]),
        );
    });

    it("takes the row off before the server answers", async () => {
        let answer: () => void = () => {};
        deleteCampaign.mockImplementationOnce(
            (id: string) =>
                new Promise<void>((resolve) => {
                    answer = () => {
                        server = server.filter((c) => c.id !== id);
                        resolve();
                    };
                }),
        );
        const list = renderHook(() => useCampaigns({ query: "", folder: "" }), { wrapper: wrapper(client) });
        await waitFor(() => expect(list.result.current.isSuccess).toBe(true));

        const del = renderHook(() => useDeleteCampaign(), { wrapper: wrapper(client) });
        const pending = del.result.current.mutateAsync("camp-a");
        await waitFor(() => expect(cachedIds(client)).toEqual(["camp-b"]));
        expect(list.result.current.campaigns.map((c) => c.id)).toEqual(["camp-b"]);

        answer();
        await pending;
        await waitFor(() => expect(getCampaigns).toHaveBeenCalledTimes(2));
        expect(cachedIds(client)).toEqual(["camp-b"]);
    });

    it("surfaces a failed delete and puts the row back", async () => {
        deleteCampaign.mockRejectedValueOnce(new Error("campaign is locked"));
        const list = renderHook(() => useCampaigns({ query: "", folder: "" }), { wrapper: wrapper(client) });
        await waitFor(() => expect(list.result.current.isSuccess).toBe(true));

        const del = renderHook(() => useDeleteCampaign(), { wrapper: wrapper(client) });
        await expect(del.result.current.mutateAsync("camp-a")).rejects.toThrow("campaign is locked");
        // Restored from the snapshot at once, not by the resync refetch.
        expect(cachedIds(client)).toEqual(["camp-a", "camp-b"]);
        await waitFor(() => expect(list.result.current.campaigns.map((c) => c.id)).toEqual(["camp-a", "camp-b"]));
    });
});
