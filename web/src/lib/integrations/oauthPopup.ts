// Drives the OAuth connect popup for third-party integrations. Mirrors the
// mailbox-onboarding flow: we open the provider authorization URL in a centered
// popup; the backend's /integrations/oauth/callback page postMessages the
// {code, state} back to this opener; we resolve with them so the caller can
// finish the handshake. The window-name carries no secret — the CSRF/PKCE
// state lives server-side, keyed by the `state` nonce.

import { API_URL } from "@/lib/information";
import { POPUP_CLOSED, closePopup, navigatePopup, notifyPopupBlocked, reservePopup, waitForPopupMessage } from "@/lib/popup";

export interface OAuthPopupResult {
    code: string;
    state: string;
}

const POPUP_MESSAGE_SOURCE = "warmbly-integration-oauth";
const WINDOW_NAME = "warmbly_oauth";
const SIZE = { width: 600, height: 720 };

// The callback page is served by the API, so only its origin (or ours, when the
// API sits behind the dashboard's origin) may hand back a code.
function callbackOrigins(): string[] {
    const origins = [window.location.origin];
    try {
        origins.push(new URL(API_URL, window.location.href).origin);
    } catch {
        /* unset API_URL leaves only our own origin */
    }
    return origins;
}

// The state the server put in the authorization URL; a callback carrying any other is not this flow's.
function issuedState(url: string): string | null {
    try {
        return new URL(url, window.location.href).searchParams.get("state");
    } catch {
        return null;
    }
}

function acceptCallback(event: MessageEvent, expectedState: string | null): OAuthPopupResult | undefined {
    if (!callbackOrigins().includes(event.origin)) return undefined;
    const data = event.data as { source?: string; code?: string; state?: string; error?: string } | undefined;
    if (!data || data.source !== POPUP_MESSAGE_SOURCE) return undefined;
    if (expectedState && data.state !== expectedState) return undefined;
    if (data.error) throw new Error(data.error);
    if (data.code && data.state) return { code: data.code, state: data.state };
    throw new Error("Authorization was cancelled.");
}

/**
 * Runs a whole connect: call it straight from the click. The window opens
 * before start() is awaited (Safari blocks one opened after), then follows the
 * authorization URL start() returns. onOpened runs once the provider page is
 * loading, or once a blocked window is waiting to be allowed.
 */
export async function authorizeInPopup(start: () => Promise<string>, onOpened?: () => void): Promise<OAuthPopupResult> {
    const reserved = reservePopup(WINDOW_NAME, SIZE);
    let closedTimer: number | undefined;
    const request = start();
    // A request that loses the race to a closed window still settles; keep its failure handled.
    request.catch(() => {});
    let url: string;
    try {
        url = await Promise.race([
            request,
            new Promise<never>((_, reject) => {
                if (!reserved) return;
                closedTimer = window.setInterval(() => {
                    if (reserved.closed) reject(new Error(POPUP_CLOSED));
                }, 500);
            }),
        ]);
    } catch (err) {
        closePopup(reserved);
        throw err;
    } finally {
        window.clearInterval(closedTimer);
    }
    const opened = navigatePopup(reserved, url, WINDOW_NAME, SIZE);
    if (opened.status === "closed") throw new Error(POPUP_CLOSED);
    if (opened.status === "blocked") notifyPopupBlocked();
    onOpened?.();
    const expectedState = issuedState(url);
    return waitForPopupMessage(opened.status === "open" ? opened.window : null, (event) => acceptCallback(event, expectedState));
}
