// Select mode on a phone: tapping Select by accident has to have a way back,
// because there is no Escape key to leave it with.

import React from "react";
import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { screen, act, fireEvent } from "@testing-library/react";
import {
    installLayoutShims,
    mount,
    resetScrollTops,
    setViewportWidth,
    settle,
    SUITE,
} from "./uniboxHarness";

type Call = { method?: string; url?: string; data?: Record<string, unknown> };

const calls = vi.hoisted((): Call[] => []);

beforeAll(() => {
    installLayoutShims();
    setViewportWidth(390);
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

async function tap(el: Element) {
    await act(async () => {
        fireEvent.click(el);
    });
    await settle();
}

describe("unibox select mode", SUITE, () => {
    beforeEach(() => {
        calls.length = 0;
        resetScrollTops();
        setViewportWidth(390);
    });

    it("leaves select mode from the header without ticking anything", async () => {
        await mount("/app/unibox/inbox");
        await settle();

        await tap(screen.getByRole("button", { name: "Select" }));
        expect(screen.getByLabelText("Select all loaded")).toBeTruthy();

        await tap(screen.getByRole("button", { name: "Cancel selection" }));
        expect(screen.queryByLabelText("Select all loaded")).toBeNull();
        expect(screen.getByRole("button", { name: "Select" })).toBeTruthy();
    });

    it("clears a selection along with the mode", async () => {
        await mount("/app/unibox/inbox");
        await settle();

        await tap(screen.getByRole("button", { name: "Select" }));
        await tap(screen.getByLabelText("Select all loaded"));
        expect(screen.getByRole("toolbar", { name: "Selection actions" })).toBeTruthy();

        await tap(screen.getByRole("button", { name: "Cancel selection" }));
        await settle();
        expect(screen.queryByRole("toolbar", { name: "Selection actions" })).toBeNull();
        expect(screen.getByRole("button", { name: "Select" })).toBeTruthy();
    });
});
