// Drives the mailbox OAuth popup outside AddEmailModal (the reconnect flow in
// the mailbox drawer). Opens the provider authorization URL in a centered
// popup; the backend's /addresses/<provider>/callback page postMessages
// {type:"email_oauth_callback", code, state} back to this opener; we resolve
// with them so the caller can finish the handshake.

import { API_URL, APP_URL } from "@/lib/information";
import { POPUP_CLOSED, closePopup, navigatePopup, notifyPopupBlocked, reservePopup, waitForPopupMessage } from "@/lib/popup";

export interface EmailOAuthPopupResult {
    code: string;
    state: string;
}

interface EmailOAuthCallbackMessage {
    type: "email_oauth_callback";
    provider: string;
    code: string;
    state: string;
    error: string;
}

const WINDOW_NAME = "warmbly_email_oauth";

// originOf normalises a configured base URL to a bare origin. APP_URL and
// API_URL may carry a trailing slash or a path; event.origin never does.
function originOf(value: string | undefined): string | null {
    if (!value) return null;
    try {
        return new URL(value, window.location.href).origin;
    } catch {
        return null;
    }
}

// The bridge page is served by the API so the registered redirect_uri stays
// stable, which means event.origin can be API_URL's origin on split-domain
// deployments. The real replay protection is the single-use state match.
function allowedCallbackOrigins(): string[] {
    return [originOf(APP_URL), originOf(API_URL), window.location.origin].filter(
        (o): o is string => Boolean(o),
    );
}

/**
 * Runs a whole mailbox authorization: call it straight from the click. The
 * window opens before start() is awaited (Safari blocks one opened after),
 * then follows the URL start() returns; resolves with the callback's code.
 */
export async function authorizeEmailInPopup(
    start: () => Promise<{ url: string; state: string }>,
): Promise<EmailOAuthPopupResult> {
    const reserved = reservePopup(WINDOW_NAME);
    let begun: { url: string; state: string };
    try {
        begun = await start();
    } catch (err) {
        closePopup(reserved);
        throw err;
    }
    const expectedState = begun.state;
    const opened = navigatePopup(reserved, begun.url, WINDOW_NAME);
    if (opened.status === "closed") throw new Error(POPUP_CLOSED);
    if (opened.status === "blocked") notifyPopupBlocked();

    return waitForPopupMessage(opened.status === "open" ? opened.window : null, (event) => {
        if (event.origin && !allowedCallbackOrigins().includes(event.origin)) return undefined;
        const data = event.data as EmailOAuthCallbackMessage | undefined;
        if (!data || data.type !== "email_oauth_callback") return undefined;
        // An admin grant state never belongs to a mailbox re-authorization.
        if (data.state !== expectedState || data.state.startsWith("mac_") || data.state.startsWith("gac_")) return undefined;
        if (data.error) {
            throw new Error(data.error === "access_denied" ? "Authorization was cancelled." : `Provider error: ${data.error}`);
        }
        if (data.code) return { code: data.code, state: data.state };
        throw new Error("Authorization was cancelled.");
    });
}
