// Drives the OAuth connect popup for third-party integrations. Mirrors the
// mailbox-onboarding flow: we open the provider authorization URL in a centered
// popup; the backend's /integrations/oauth/callback page postMessages the
// {code, state} back to this opener; we resolve with them so the caller can
// finish the handshake. The window-name carries no secret — the CSRF/PKCE
// state lives server-side, keyed by the `state` nonce.

import { API_URL } from "@/lib/information";

export interface OAuthPopupResult {
    code: string;
    state: string;
}

const POPUP_MESSAGE_SOURCE = "warmbly-integration-oauth";

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

function popupFeatures(): string {
    const width = 600;
    const height = 720;
    const left = window.screenX + Math.max(0, (window.outerWidth - width) / 2);
    const top = window.screenY + Math.max(0, (window.outerHeight - height) / 2);
    return `width=${width},height=${height},left=${left},top=${top},menubar=no,toolbar=no,location=yes`;
}

const POPUP_BLOCKED = "Popup blocked. Allow popups for this site and try again.";
const POPUP_CLOSED = "Authorization window was closed before finishing.";

// Opens the window inside the click, before start() is awaited: a browser
// blocks a popup opened after the click's activation has lapsed. onOpened runs
// once the provider page is loading in it.
export async function authorizeInPopup(start: () => Promise<string>, onOpened?: () => void): Promise<OAuthPopupResult> {
    const popup = window.open("about:blank", "warmbly_oauth", popupFeatures());
    if (!popup) throw new Error(POPUP_BLOCKED);
    let closedTimer: number | undefined;
    const request = start();
    // A request that loses the race to a closed window still settles; keep its failure handled.
    request.catch(() => {});
    let url: string;
    try {
        url = await Promise.race([
            request,
            new Promise<never>((_, reject) => {
                closedTimer = window.setInterval(() => {
                    if (popup.closed) reject(new Error(POPUP_CLOSED));
                }, 600);
            }),
        ]);
    } catch (err) {
        popup.close();
        throw err;
    } finally {
        window.clearInterval(closedTimer);
    }
    if (popup.closed) throw new Error(POPUP_CLOSED);
    onOpened?.();
    return openOAuthPopup(url, popup);
}

export function openOAuthPopup(authUrl: string, reserved?: Window): Promise<OAuthPopupResult> {
    return new Promise((resolve, reject) => {
        const popup = reserved ?? window.open(authUrl, "warmbly_oauth", popupFeatures());
        if (!popup) {
            reject(new Error(POPUP_BLOCKED));
            return;
        }
        if (reserved) reserved.location.href = authUrl;

        let settled = false;
        const cleanup = () => {
            window.removeEventListener("message", onMessage);
            window.clearInterval(closedTimer);
        };

        const onMessage = (event: MessageEvent) => {
            if (!callbackOrigins().includes(event.origin)) return;
            const data = event.data as
                | { source?: string; code?: string; state?: string; error?: string }
                | undefined;
            if (!data || data.source !== POPUP_MESSAGE_SOURCE) return;
            settled = true;
            cleanup();
            try {
                popup.close();
            } catch {
                /* ignore */
            }
            if (data.error) {
                reject(new Error(data.error));
                return;
            }
            if (data.code && data.state) {
                resolve({ code: data.code, state: data.state });
                return;
            }
            reject(new Error("Authorization was cancelled."));
        };

        window.addEventListener("message", onMessage);

        // Detect a manually-closed popup so the caller's promise doesn't hang.
        const closedTimer = window.setInterval(() => {
            if (popup.closed && !settled) {
                cleanup();
                reject(new Error(POPUP_CLOSED));
            }
        }, 600);
    });
}
