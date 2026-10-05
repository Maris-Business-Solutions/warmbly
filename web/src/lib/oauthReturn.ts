// A sign-in window the browser opened without a link back to the dashboard
// (one allowed from a blocked-popup prompt) lands on /oauth-return. That page
// hands the result to the waiting tab over a same-origin BroadcastChannel,
// and the tab replays it as a message from its own origin, so every flow's
// existing origin and state checks decide whether it is theirs.

const CHANNEL = "warmbly-oauth-return";

export type OAuthReturnMessage =
    | { type: "email_oauth_callback"; provider: string; code: string; state: string; error: string }
    | { source: "warmbly-integration-oauth"; code: string; state: string; error: string }
    | { type: "cloud_oauth_callback"; session: string; status: "ok" | "error"; error?: string; message?: string };

/** The message the callback page put in the fragment of /oauth-return, or null when it is not one. */
export function oauthReturnMessage(hash: string): OAuthReturnMessage | null {
    const p = new URLSearchParams(hash.replace(/^#/, ""));
    const code = p.get("code") ?? "";
    const state = p.get("state") ?? "";
    const error = p.get("error") ?? "";
    if (!state) return null;
    switch (p.get("source")) {
        case "mailbox":
            return { type: "email_oauth_callback", provider: p.get("provider") ?? "", code, state, error };
        case "integration":
            return { source: "warmbly-integration-oauth", code, state, error };
        default:
            return null;
    }
}

/** Sends a sign-in result to every dashboard tab; false when the browser has no BroadcastChannel. */
export function relayOAuthReturn(message: OAuthReturnMessage): boolean {
    if (typeof BroadcastChannel === "undefined") return false;
    const ch = new BroadcastChannel(CHANNEL);
    ch.postMessage(message);
    ch.close();
    return true;
}

let listening = false;

/** Replays results relayed by /oauth-return as window messages. Idempotent. */
export function listenForOAuthReturn(): void {
    if (listening || typeof BroadcastChannel === "undefined") return;
    listening = true;
    new BroadcastChannel(CHANNEL).onmessage = (event: MessageEvent) => {
        window.dispatchEvent(new MessageEvent("message", { data: event.data, origin: window.location.origin }));
    };
}
