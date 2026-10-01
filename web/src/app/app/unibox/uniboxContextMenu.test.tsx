// The conversation row's right-click menu: the row's own actions at the
// pointer, and the whole selection when the row is one of several ticked.

import React from "react";
import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { screen, act, fireEvent, waitFor, within } from "@testing-library/react";
import {
    installLayoutShims,
    mount,
    resetScrollTops,
    ROWS,
    setViewportWidth,
    settle,
    SUITE,
} from "./uniboxHarness";

type Call = { method?: string; url?: string; data?: Record<string, unknown> };

const calls = vi.hoisted((): Call[] => []);

beforeAll(() => {
    installLayoutShims();
    setViewportWidth(1512);
});

vi.mock("@/lib/api/client/Request", () => ({
    default: async (cfg: Call) => {
        calls.push({ method: cfg.method, url: cfg.url, data: cfg.data });
        const { route } = await import("./uniboxHarness");
        return route(String(cfg?.url ?? ""));
    },
}));
vi.mock("@/lib/helper/getToken", () => ({
    default: () => ({
        access_token: "a",
        refresh_token: "r",
        access_token_expires_at: new Date(Date.now() + 3600e3).toISOString(),
        refresh_token_expires_at: new Date(Date.now() + 3600e3).toISOString(),
    }),
}));
vi.mock("@/hooks/SocketProvider", () => ({
    default: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
vi.mock("@/hooks/context/socket", async (orig) => {
    const actual = (await orig()) as Record<string, unknown>;
    return {
        ...actual,
        useSocket: () => ({
            isConnected: false,
            subscribeToChannel: () => () => {},
            pushToChannel: () => {},
            socket: null,
            status: "closed",
        }),
        useChannel: () => ({ state: "closed", push: () => {}, channel: null }),
        useChannelEvent: () => {},
        useChannelSubscription: () => {},
    };
});

const patches = (path: string) =>
    calls.filter((c) => c.method === "PATCH" && c.url === path);

// The open reader repeats the subject, so take the copy that sits in a row.
const rowFor = (i: number) =>
    screen
        .getAllByText(ROWS[i].subject)
        .map((el) => el.closest<HTMLElement>('[role="button"]'))
        .find((row) => row?.querySelector('[aria-label="Conversation actions"]'))!;

async function rightClick(i: number) {
    await act(async () => {
        fireEvent.contextMenu(rowFor(i), { clientX: 120, clientY: 160 });
    });
    await settle();
}

async function choose(name: string) {
    await act(async () => {
        fireEvent.click(screen.getByRole("menuitem", { name }));
    });
    await settle();
}

async function tick(i: number) {
    await act(async () => {
        fireEvent.click(rowFor(i).querySelector('input[type="checkbox"]')!);
    });
}

describe("unibox conversation context menu", SUITE, () => {
    beforeEach(() => {
        calls.length = 0;
        resetScrollTops();
        setViewportWidth(1512);
    });

    // The same actions the row's "…" offers, and the row stays closed.
    it("opens the row's menu on a right-click without opening the conversation", async () => {
        const router = await mount("/app/unibox/awaiting");
        await settle();

        await rightClick(0);

        const menu = screen.getByRole("menu");
        for (const name of ["Open in new tab", "Mark as unread", "Snooze…", "Labels…", "Archive", "Delete"]) {
            expect(within(menu).getByRole("menuitem", { name })).toBeTruthy();
        }
        expect(router.state.location.pathname).toBe("/app/unibox/awaiting");
    });

    it("archives the right-clicked conversation", async () => {
        await mount("/app/unibox/awaiting");
        await settle();

        await rightClick(2);
        calls.length = 0;
        await choose("Archive");

        const [filed] = patches("/unibox/folder");
        expect(filed).toBeTruthy();
        expect(filed.data).toMatchObject({ folder: "archive", thread_ids: [ROWS[2].thread_id] });
    });

    it("offers the way back in Trash", async () => {
        await mount("/app/unibox/trash");
        await settle();

        await rightClick(0);

        const menu = screen.getByRole("menu");
        expect(within(menu).getByRole("menuitem", { name: "Move to inbox" })).toBeTruthy();
        expect(within(menu).queryByRole("menuitem", { name: "Delete" })).toBeNull();
    });

    // A right-click on one of several ticked rows means all of them, as in a
    // file list, and the ticks go once it is done.
    it("acts on the whole selection when a ticked row is right-clicked", async () => {
        await mount("/app/unibox/awaiting");
        await settle();
        await tick(0);
        await tick(1);
        await settle();

        await rightClick(1);
        expect(screen.getByText("2 conversations")).toBeTruthy();
        expect(screen.queryByRole("menuitem", { name: "Open in new tab" })).toBeNull();
        calls.length = 0;
        await choose("Archive");

        const filed = patches("/unibox/folder");
        expect(filed).toHaveLength(1);
        expect(filed[0].data).toMatchObject({
            folder: "archive",
            thread_ids: [ROWS[0].thread_id, ROWS[1].thread_id],
        });
        await waitFor(() => expect(screen.queryByText("2 selected")).toBeNull());
    });

    it("acts on just that row when an unticked row is right-clicked", async () => {
        await mount("/app/unibox/awaiting");
        await settle();
        await tick(0);
        await tick(1);
        await settle();

        await rightClick(4);
        calls.length = 0;
        await choose("Archive");

        const [filed] = patches("/unibox/folder");
        expect(filed.data).toMatchObject({ thread_ids: [ROWS[4].thread_id] });
        expect(screen.getByText("2 selected")).toBeTruthy();
    });

    it("opens the conversation in a new tab at its own address", async () => {
        const open = vi.spyOn(window, "open").mockImplementation(() => null);
        await mount("/app/unibox/awaiting");
        await settle();

        await rightClick(3);
        await choose("Open in new tab");

        expect(open).toHaveBeenCalledWith(
            `/app/unibox/awaiting/${ROWS[3].thread_id}`,
            "_blank",
            "noopener",
        );
        open.mockRestore();
    });

    // The reader marks whatever it shows as read, so leaving it open would
    // undo the click.
    it("closes the open conversation when it is marked unread", async () => {
        const router = await mount(`/app/unibox/awaiting/${ROWS[0].thread_id}`);
        await settle();

        await rightClick(0);
        calls.length = 0;
        await choose("Mark as unread");

        const [seen] = patches("/unibox/seen");
        expect(seen.data).toMatchObject({ seen: false, thread_ids: [ROWS[0].thread_id] });
        await waitFor(() => expect(router.state.location.pathname).toBe("/app/unibox/awaiting"));
    });

    it("labels from the menu without opening the conversation", async () => {
        const router = await mount("/app/unibox/awaiting");
        await settle();

        await rightClick(0);
        await act(async () => {
            fireEvent.click(screen.getByRole("menuitem", { name: "Labels…" }));
        });
        await settle();

        expect(screen.getByPlaceholderText("Label conversation…")).toBeTruthy();
        expect(router.state.location.pathname).toBe("/app/unibox/awaiting");
    });

    it("keeps one menu open across right-clicks and closes on an outside press", async () => {
        await mount("/app/unibox/awaiting");
        await settle();

        await rightClick(0);
        await act(async () => {
            fireEvent.pointerDown(rowFor(1), { button: 2 });
        });
        await rightClick(1);
        expect(screen.getAllByRole("menu")).toHaveLength(1);

        await act(async () => {
            fireEvent.pointerDown(document.body);
        });
        await settle();
        expect(screen.queryByRole("menu")).toBeNull();
    });
});
