// useAdminGrantPopup: the sign-in window an administrator completes to grant a
// whole Google Workspace domain ("gac_" states) or Microsoft 365 tenant ("mac_").
// The popup ends on the mailbox OAuth callback page, which posts
// {type:"email_oauth_callback", code, state}; only the state this hook opened is
// ours, and useMailboxOAuth ignores both prefixes.

import React from "react";
import toast from "react-hot-toast";
import { useMutation } from "@tanstack/react-query";
import { useStoreGrant } from "@/lib/api/hooks/app/emails/useMailboxGrants";
import { sourceMutationKey } from "@/lib/api/hooks/app/emails/mailboxSourceBusy";
import type { DomainGrant } from "@/lib/api/models/app/emails/MailboxSources";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { allowedCallbackOrigins, isAdminConsentState } from "@/hooks/useMailboxOAuth";
import { BLOCKED_WAIT_MS, closePopup, navigatePopup, notifyPopupBlocked, reservePopup } from "@/lib/popup";

interface CallbackMessage {
    type: "email_oauth_callback";
    code: string;
    state: string;
    error: string;
}

/** A grant refusal in words a person can act on. */
export function grantErrorText(e: AppError, provider: "Google" | "Microsoft"): string {
    if (e?.code === "mailbox_grant_proof_missing") {
        return provider === "Google"
            ? `${buildError(e)} If you just added the DNS record, give it a few minutes and check again. Signing in as the super admin works too.`
            : `${buildError(e)} Sign in with a Global Administrator of the organization.`;
    }
    if (e?.code === "google_delegation_unauthorized") {
        return `${buildError(e)} A new delegation can take a few minutes to reach every Google server, so if you just added it, wait and try again.`;
    }
    if (e?.code === "microsoft_consent_missing") {
        return `${buildError(e)} The consent screen has to be accepted for the whole organization.`;
    }
    if (e?.code === "mailbox_grant_state_invalid") return buildError(e) || "This sign-in expired. Start again.";
    return buildError(e);
}

export default function useAdminGrantPopup({
    provider,
    windowName,
    finish,
    onGranted,
    onError,
}: {
    provider: "Google" | "Microsoft";
    windowName: string;
    finish: (body: { state: string; code: string }) => Promise<DomainGrant>;
    onGranted?: (grant: DomainGrant) => void;
    /** `code` is the API's error code when the refusal came from the server. */
    onError?: (message: string, code?: string) => void;
}) {
    const store = useStoreGrant();
    const [busy, setBusy] = React.useState(false);
    const pending = React.useRef<string | null>(null);
    // A sign-in nobody finishes stops being honoured after BLOCKED_WAIT_MS.
    const expiry = React.useRef<number | undefined>(undefined);
    React.useEffect(() => () => window.clearTimeout(expiry.current), []);
    const popupRef = React.useRef<Window | null>(null);

    const cb = React.useRef({ finish, onGranted, onError });
    React.useEffect(() => {
        cb.current = { finish, onGranted, onError };
    });

    // Keyed so the dialog around it cannot be closed while the grant is recorded.
    const { mutateAsync: finishAsync } = useMutation({
        mutationKey: sourceMutationKey(`grant-${provider.toLowerCase()}-signin`),
        mutationFn: (body: { state: string; code: string }) => cb.current.finish(body),
    });

    React.useEffect(() => {
        function onMessage(event: MessageEvent) {
            if (event.origin && !allowedCallbackOrigins().includes(event.origin)) return;
            const data = event.data as CallbackMessage | undefined;
            if (!data || data.type !== "email_oauth_callback" || !isAdminConsentState(data.state)) return;
            if (!pending.current || pending.current !== data.state) return;
            pending.current = null;
            popupRef.current = null;

            if (data.error || !data.code) {
                setBusy(false);
                if (data.error !== "access_denied") {
                    const msg = data.error ? `${provider} answered: ${data.error}` : "The sign-in was cancelled.";
                    toast.error(msg);
                    cb.current.onError?.(msg);
                }
                return;
            }
            void toast
                .promise(
                    finishAsync({ state: data.state, code: data.code }).then((grant) => {
                        store(grant);
                        cb.current.onGranted?.(grant);
                        return grant;
                    }),
                    {
                        loading: `Checking the grant with ${provider}…`,
                        success: (g: DomainGrant) => `${g.provider === "google" ? g.tenant : "Microsoft 365 organization"} connected`,
                        error: (e: AppError) => {
                            const msg = grantErrorText(e, provider);
                            cb.current.onError?.(msg, e?.code);
                            return msg;
                        },
                    },
                )
                .catch(() => undefined)
                .finally(() => setBusy(false));
        }
        window.addEventListener("message", onMessage);
        return () => window.removeEventListener("message", onMessage);
    }, [store, provider, finishAsync]);

    // Closing the window stops the spinner. The pending state stays, so a
    // callback that lands just after the window closes still finishes.
    React.useEffect(() => {
        if (!busy) return;
        let grace: number | undefined;
        const t = window.setInterval(() => {
            const p = popupRef.current;
            if (!p || !p.closed || grace !== undefined) return;
            grace = window.setTimeout(() => {
                if (popupRef.current === p) {
                    popupRef.current = null;
                    setBusy(false);
                }
            }, 1000);
        }, 500);
        return () => {
            window.clearInterval(t);
            window.clearTimeout(grace);
        };
    }, [busy]);

    /** Opens the sign-in window now, inside the click, for a caller that has to await before open(). */
    const reserve = React.useCallback(() => reservePopup(windowName), [windowName]);

    /**
     * Asks the server for a fresh sign-in URL and state, then opens it. Call it
     * straight from the click, or pass the window reserve() opened there:
     * Safari blocks a window opened after an await.
     */
    const open = React.useCallback(
        async (begin: () => Promise<{ url?: string; state?: string }>, reserved?: Window | null) => {
            if (busy) {
                closePopup(reserved);
                return;
            }
            const win = reserved === undefined ? reservePopup(windowName) : reserved;
            setBusy(true);
            pending.current = null;
            try {
                const { url, state } = await begin();
                if (!url || !state) throw new Error("no sign-in");
                pending.current = state;
                window.clearTimeout(expiry.current);
                expiry.current = window.setTimeout(() => {
                    if (pending.current === state) pending.current = null;
                }, BLOCKED_WAIT_MS);
                const opened = navigatePopup(win, url, windowName);
                if (opened.status === "open") {
                    popupRef.current = opened.window;
                    return;
                }
                popupRef.current = null;
                setBusy(false);
                if (opened.status === "closed") {
                    pending.current = null;
                    return;
                }
                // Still listening: a window allowed from the address bar finishes the grant.
                notifyPopupBlocked();
            } catch (e) {
                closePopup(win);
                pending.current = null;
                setBusy(false);
                const msg = e instanceof Error ? `Could not start the ${provider} sign-in. Try again.` : grantErrorText(e as AppError, provider);
                toast.error(msg);
                cb.current.onError?.(msg, e instanceof Error ? undefined : (e as AppError)?.code);
            }
        },
        [busy, provider, windowName],
    );

    const reset = React.useCallback(() => {
        pending.current = null;
        popupRef.current = null;
        setBusy(false);
    }, []);

    return { busy, open, reserve, reset };
}
