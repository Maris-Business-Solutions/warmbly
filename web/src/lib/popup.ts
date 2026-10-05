// Sign-in windows (OAuth consent popups) for every flow in the dashboard.
//
// Safari only lets a page open a window while it is still handling the click:
// await anything first, such as the request for the authorization URL, and
// the window is blocked. So the window opens inside the click on a holding
// page and is pointed at the provider once the URL arrives. When the browser
// blocks it anyway, the flow keeps listening, so a window the person then
// allows from the address bar still finishes the sign-in.

import toast from "react-hot-toast";
import { listenForOAuthReturn } from "@/lib/oauthReturn";

export interface PopupSize {
    width: number;
    height: number;
}

const DEFAULT_SIZE: PopupSize = { width: 520, height: 640 };

export const POPUP_CLOSED = "Authorization window was closed before finishing.";

// How long a blocked sign-in keeps listening for a window allowed from the address bar.
export const BLOCKED_WAIT_MS = 10 * 60 * 1000;

// The callback page posts just before it closes itself; its message may land after `closed` flips.
const CLOSE_GRACE_MS = 1000;

function features({ width, height }: PopupSize): string {
    const sx = window.screenX ?? window.screenLeft ?? 0;
    const sy = window.screenY ?? window.screenTop ?? 0;
    const ow = window.outerWidth || window.innerWidth || screen.width;
    const oh = window.outerHeight || window.innerHeight || screen.height;
    const left = Math.round(sx + Math.max(0, (ow - width) / 2));
    const top = Math.round(sy + Math.max(0, (oh - height) / 2));
    return `width=${width},height=${height},left=${left},top=${top}`;
}

const HOLDING_PAGE =
    '<p style="margin:0;height:100vh;display:flex;align-items:center;justify-content:center;' +
    'font:14px -apple-system,system-ui,sans-serif;color:#64748b;background:#f8fafc">Connecting…</p>';

/**
 * Opens a named window now, while the click is still being handled, on a
 * holding page until navigatePopup points it at the provider. Null when the
 * browser blocked it.
 */
export function reservePopup(name: string, size: PopupSize = DEFAULT_SIZE): Window | null {
    listenForOAuthReturn();
    let w: Window | null = null;
    try {
        w = window.open("", name, features(size));
    } catch {
        w = null;
    }
    if (!w) return null;
    try {
        // A window left open by an earlier attempt is reused; only a fresh one is blank.
        if (w.location.href === "about:blank" && w.document.body && !w.document.body.childElementCount) {
            w.document.title = "Connecting…";
            w.document.body.style.margin = "0";
            w.document.body.innerHTML = HOLDING_PAGE;
        }
    } catch {
        /* an earlier attempt's window on the provider's origin; navigatePopup replaces it */
    }
    try {
        w.focus();
    } catch {
        /* ignore */
    }
    return w;
}

export type PopupOpen = { status: "open"; window: Window } | { status: "blocked" } | { status: "closed" };

/**
 * Points a reserved window at `url`. Without one (the browser blocked it), it
 * asks for the window again with the real address, so the browser's blocked
 * window prompt opens the provider rather than a blank page.
 */
export function navigatePopup(reserved: Window | null, url: string, name: string, size: PopupSize = DEFAULT_SIZE): PopupOpen {
    if (reserved) {
        if (reserved.closed) return { status: "closed" };
        let moved = false;
        try {
            reserved.location.replace(url);
            moved = true;
        } catch {
            /* a torn-down window; reopen by name below */
        }
        if (moved) {
            try {
                reserved.focus();
            } catch {
                /* ignore */
            }
            return { status: "open", window: reserved };
        }
    }
    let w: Window | null = null;
    try {
        w = window.open(url, name, features(size));
    } catch {
        w = null;
    }
    if (!w) return { status: "blocked" };
    try {
        w.focus();
    } catch {
        /* ignore */
    }
    return { status: "open", window: w };
}

export function closePopup(w: Window | null | undefined) {
    try {
        if (w && !w.closed) w.close();
    } catch {
        /* ignore */
    }
}

/** Tells the person how to finish a sign-in whose window the browser blocked. */
export function notifyPopupBlocked() {
    toast.error(
        "Your browser blocked the sign-in window. Allow it from the address bar and finish signing in there; this page picks it up when you're done.",
        { id: "popup-blocked", duration: 12000 },
    );
}

/**
 * Settles with the first message `accept` recognises (anything but undefined),
 * closing the window. Rejects with POPUP_CLOSED once `popup` closes without one;
 * with no window to watch (blocked) it listens for BLOCKED_WAIT_MS.
 */
export function waitForPopupMessage<T>(popup: Window | null, accept: (event: MessageEvent) => T | undefined): Promise<T> {
    return new Promise<T>((resolve, reject) => {
        let settled = false;
        let graceTimer: number | undefined;
        const finish = () => {
            settled = true;
            window.removeEventListener("message", onMessage);
            window.clearInterval(closedTimer);
            window.clearTimeout(graceTimer);
            window.clearTimeout(blockedTimer);
        };
        const onMessage = (event: MessageEvent) => {
            if (settled) return;
            let value: T | undefined;
            try {
                value = accept(event);
            } catch (err) {
                finish();
                closePopup(popup);
                reject(err);
                return;
            }
            if (value === undefined) return;
            finish();
            closePopup(popup);
            resolve(value);
        };
        window.addEventListener("message", onMessage);

        const closedTimer = popup
            ? window.setInterval(() => {
                  if (settled || graceTimer !== undefined || !popup.closed) return;
                  graceTimer = window.setTimeout(() => {
                      if (settled) return;
                      finish();
                      reject(new Error(POPUP_CLOSED));
                  }, CLOSE_GRACE_MS);
              }, 500)
            : undefined;
        const blockedTimer = popup
            ? undefined
            : window.setTimeout(() => {
                  if (settled) return;
                  finish();
                  reject(new Error("The sign-in was not finished. Try again."));
              }, BLOCKED_WAIT_MS);
    });
}
