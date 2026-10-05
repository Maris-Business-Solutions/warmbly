// A sign-in window allowed from a blocked-popup prompt has no opener; its result
// reaches the waiting tab through /oauth-return and the BroadcastChannel relay.

import { describe, expect, it, vi } from "vitest";
import { listenForOAuthReturn, oauthReturnMessage, relayOAuthReturn } from "./oauthReturn";

describe("oauthReturnMessage", () => {
    it("rebuilds the mailbox callback message", () => {
        expect(oauthReturnMessage("#source=mailbox&provider=gmail&code=c%2F1&state=w.s&error=")).toEqual({
            type: "email_oauth_callback",
            provider: "gmail",
            code: "c/1",
            state: "w.s",
            error: "",
        });
    });

    it("rebuilds the integration callback message", () => {
        expect(oauthReturnMessage("#source=integration&code=c&state=s&error=")).toEqual({
            source: "warmbly-integration-oauth",
            code: "c",
            state: "s",
            error: "",
        });
    });

    it("ignores a fragment without a state or a known source", () => {
        expect(oauthReturnMessage("#source=mailbox&code=c")).toBeNull();
        expect(oauthReturnMessage("#source=other&code=c&state=s")).toBeNull();
        expect(oauthReturnMessage("")).toBeNull();
    });
});

describe("relayOAuthReturn", () => {
    it("replays a relayed result as a message from the dashboard's own origin", async () => {
        listenForOAuthReturn();
        listenForOAuthReturn();
        const seen = vi.fn();
        const onMessage = (e: MessageEvent) => seen(e.origin, e.data);
        window.addEventListener("message", onMessage);
        const msg = oauthReturnMessage("#source=mailbox&provider=outlook&code=c&state=w.s")!;
        expect(relayOAuthReturn(msg)).toBe(true);
        await vi.waitFor(() => expect(seen).toHaveBeenCalledTimes(1));
        expect(seen).toHaveBeenCalledWith(window.location.origin, msg);
        window.removeEventListener("message", onMessage);
    });
});
