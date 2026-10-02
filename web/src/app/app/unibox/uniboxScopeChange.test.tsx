// Issue #741: switching scope carried the open conversation into a list that
// did not have it, with no way to close it on desktop but Escape.

import React from "react";
import { describe, it, expect, vi, beforeAll } from "vitest";
import { screen, act, fireEvent } from "@testing-library/react";
import { findThreadRow, installLayoutShims, mount, settle, SUITE } from "./uniboxHarness";

beforeAll(installLayoutShims);

vi.mock("@/lib/api/client/Request", () => ({
    default: async (cfg: { url?: string }) => {
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

async function openThread(subject: string) {
    const row = await findThreadRow(subject);
    await act(async () => {
        fireEvent.click(row);
    });
    await settle();
}

describe("unibox scope change", SUITE, () => {
    it("closes the open conversation when another scope is picked", async () => {
        const router = await mount("/app/unibox/all");
        await settle();
        await openThread("Subject 4");
        expect(router.state.location.pathname).toBe("/app/unibox/all/thread-4");

        await act(async () => {
            fireEvent.click(screen.getAllByTitle("Unread")[0]);
        });
        await settle();

        expect(router.state.location.pathname).toBe("/app/unibox/unread");
        expect(screen.queryByText("No conversation open")).toBeTruthy();
    });

    it("closes the conversation from the reader's own close button", async () => {
        const router = await mount("/app/unibox/all");
        await settle();
        await openThread("Subject 2");
        expect(router.state.location.pathname).toBe("/app/unibox/all/thread-2");

        await act(async () => {
            fireEvent.click(screen.getByRole("button", { name: "Close conversation" }));
        });
        await settle();

        expect(router.state.location.pathname).toBe("/app/unibox/all");
    });
});
