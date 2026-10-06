// The scope rail follows the member across devices: what the account holds
// is applied, edits go up, and a rail from another workspace never does.

import React from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { UniboxRailLayout, ViewPreferences } from "@/lib/api/models/app/views/ViewPreferences";

const server = vi.hoisted(() => ({
    saved: null as null | { layout: unknown },
    puts: [] as unknown[],
}));

vi.mock("@/lib/api/client/app/views/views", () => ({
    getViewPreferences: async () => ({
        preferences: server.saved
            ? { view: "unibox_rail", columns: [], layout: server.saved.layout, updated_at: new Date() }
            : { view: "unibox_rail", columns: [] },
    }),
    updateViewPreferences: async (_view: string, body: { layout: unknown }) => {
        server.puts.push(body.layout);
        server.saved = { layout: body.layout };
        return { preferences: { view: "unibox_rail", columns: [], layout: body.layout, updated_at: new Date() } as ViewPreferences };
    },
}));
vi.mock("react-hot-toast/headless", () => ({ default: { error: () => {}, success: () => {} } }));

import { useAppStore } from "@/stores/useAppStore";
import { useUniboxRailSync } from "./useUniboxRailSync";

const OWNER = "u1:org-1";

function mount() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return renderHook(() => useUniboxRailSync(), {
        wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
    });
}

function setRail(l: Partial<UniboxRailLayout>, owner: string | null) {
    useAppStore.setState({
        uniboxRailFavorites: l.favorites ?? [],
        uniboxRailHidden: l.hidden ?? [],
        uniboxRailOrder: l.order ?? {},
        uniboxRailSectionOrder: l.section_order ?? [],
        uniboxRailOwner: owner,
    });
}

describe("useUniboxRailSync", () => {
    beforeEach(() => {
        server.saved = null;
        server.puts = [];
        useAppStore.setState({
            user: { id: "u1" } as never,
            currentOrganization: { id: "org-1" } as never,
        });
        setRail({}, null);
    });
    afterEach(cleanup);

    it("applies the rail the account holds over what this browser had", async () => {
        server.saved = { layout: { favorites: [{ key: "folder:inbox", name: "Work" }], hidden: ["view:today"], order: {}, section_order: [] } };
        setRail({ favorites: [{ key: "folder:sent" }] }, OWNER);
        mount();
        await waitFor(() => expect(useAppStore.getState().uniboxRailFavorites).toEqual([{ key: "folder:inbox", name: "Work" }]));
        expect(useAppStore.getState().uniboxRailHidden).toEqual(["view:today"]);
        await new Promise((r) => setTimeout(r, 800));
        expect(server.puts).toEqual([]);
    });

    it("uploads a rail arranged here before it was saved to the account", async () => {
        setRail({ favorites: [{ key: "folder:inbox" }] }, null);
        mount();
        await waitFor(() => expect(server.puts).toHaveLength(1));
        expect(server.puts[0]).toMatchObject({ favorites: [{ key: "folder:inbox" }] });
        expect(useAppStore.getState().uniboxRailOwner).toBe(OWNER);
    });

    it("starts another workspace's rail from the default instead of copying it", async () => {
        setRail({ favorites: [{ key: "folder:inbox" }], hidden: ["view:today"] }, "u1:org-2");
        mount();
        await waitFor(() => expect(useAppStore.getState().uniboxRailOwner).toBe(OWNER));
        expect(useAppStore.getState().uniboxRailFavorites).toEqual([]);
        expect(useAppStore.getState().uniboxRailHidden).toEqual([]);
        await new Promise((r) => setTimeout(r, 800));
        expect(server.puts).toEqual([]);
    });

    it("saves an edit once the rail settles", async () => {
        server.saved = { layout: { favorites: [], hidden: [], order: {}, section_order: [] } };
        mount();
        await waitFor(() => expect(useAppStore.getState().uniboxRailOwner).toBe(OWNER));

        act(() => {
            useAppStore.getState().toggleUniboxRailFavorite("folder:inbox");
            useAppStore.getState().toggleUniboxRailRow("view:today");
        });
        await waitFor(() => expect(server.puts).toHaveLength(1), { timeout: 2000 });
        expect(server.puts[0]).toMatchObject({ favorites: [{ key: "folder:inbox" }], hidden: ["view:today"] });
    });
});
