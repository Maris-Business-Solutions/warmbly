// useMailboxOAuth: the Google / Microsoft connect popup shared by the connect
// modal and the mailbox import's per-row "Sign in".
//
// The popup posts {type:"email_oauth_callback", code, state} back here and we
// finish with the user's bearer. Linked to Warmbly Cloud, the consent runs on
// the cloud's app, the popup returns to /cloud-oauth/done, and we redeem its
// session through /cloud-link/oauth/finish instead.

import React from "react";
import toast from "react-hot-toast";
import { useQueryClient } from "@tanstack/react-query";
import type Inbox from "@/lib/api/models/app/emails/Inbox";
import { API_URL, APP_URL } from "@/lib/information";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import onboardOAuthStart from "@/lib/api/client/app/emails/onboardOAuthStart";
import onboardOAuthFinish from "@/lib/api/client/app/emails/onboardOAuthFinish";
import { finishCloudOAuth, startCloudOAuth } from "@/lib/api/client/app/cloudlink/cloudLink";
import type { CloudOAuthDoneMessage } from "@/app/cloud-oauth/done/page";
import { capture } from "@/lib/productAnalytics";
import useCloudPool from "@/hooks/useCloudPool";

export type MailboxOAuthProvider = "gmail" | "outlook";

interface OAuthCallbackMessage {
    type: "email_oauth_callback";
    provider: MailboxOAuthProvider;
    code: string;
    state: string;
    error: string;
}

export interface MailboxOAuthOptions {
    /** The mailbox is connected; the caller closes or refreshes what it owns. */
    onConnected?: (provider: MailboxOAuthProvider, inbox: Inbox) => void;
    /** A connect refused because the workspace's mailbox allowance is full. */
    onAllowance?: () => void;
    /** The deployment has no OAuth client for the provider. Toasts when absent. */
    onNotConfigured?: (provider: MailboxOAuthProvider) => void;
    /** Toast copy while the finish call runs and when it lands. */
    messages?: { loading?: string; success?: string };
}

/** The one answer every connect path shares: open the allowance dialog. */
export function isAllowanceError(e: unknown): boolean {
    return (e as AppError | undefined)?.code === "mailbox_allowance_reached";
}

/** Admin grant states (Google "gac_", Microsoft "mac_"); the callback page posts them like a mailbox sign-in. */
export function isAdminConsentState(state: string | undefined | null): boolean {
    return typeof state === "string" && (state.startsWith("gac_") || state.startsWith("mac_"));
}

// APP_URL and API_URL may carry a trailing slash or a path; event.origin never does.
function originOf(value: string | undefined): string | null {
    if (!value) return null;
    try {
        return new URL(value, window.location.href).origin;
    } catch {
        return null;
    }
}

// The bridge page is served by the API, so on a split-domain deployment the
// callback arrives from API_URL's origin. This is a coarse gate: the real
// replay protection is the single-use state match.
export function allowedCallbackOrigins(): string[] {
    return [originOf(APP_URL), originOf(API_URL), window.location.origin].filter(
        (o): o is string => Boolean(o),
    );
}

export function openCentered(url: string, name: string): Window | null {
    const w = 520;
    const h = 640;
    const sx = window.screenLeft ?? window.screenX;
    const sy = window.screenTop ?? window.screenY;
    const sw = window.innerWidth ?? document.documentElement.clientWidth ?? screen.width;
    const sh = window.innerHeight ?? document.documentElement.clientHeight ?? screen.height;
    const left = sx + (sw - w) / 2;
    const top = sy + (sh - h) / 2;
    const popup = window.open(url, name, `width=${w},height=${h},left=${left},top=${top}`);
    popup?.focus();
    return popup;
}

export default function useMailboxOAuth(options: MailboxOAuthOptions = {}) {
    const qc = useQueryClient();
    const pool = useCloudPool();
    const viaCloud = pool.connected;

    const [busy, setBusy] = React.useState<MailboxOAuthProvider | null>(null);
    // Microsoft only: forwarded to an administrator when the organization requires approval.
    const [adminConsentUrl, setAdminConsentUrl] = React.useState<string | null>(null);
    const pendingState = React.useRef<{ provider: MailboxOAuthProvider; state: string } | null>(null);
    // A consent running on Warmbly Cloud's app; redeemed by session, not code.
    const pendingCloud = React.useRef<{ provider: MailboxOAuthProvider; session: string } | null>(null);

    // The listeners read the latest callbacks without re-subscribing on every render.
    const optionsRef = React.useRef(options);
    React.useEffect(() => {
        optionsRef.current = options;
    });

    const onConnectError = React.useCallback((e: unknown) => {
        if (isAllowanceError(e)) optionsRef.current.onAllowance?.();
    }, []);

    // The cloud-brokered popup lands on our own origin (/cloud-oauth/done).
    React.useEffect(() => {
        function onMessage(event: MessageEvent) {
            if (event.origin !== window.location.origin) return;
            const data = event.data as CloudOAuthDoneMessage | undefined;
            if (!data || data.type !== "cloud_oauth_callback") return;
            const expected = pendingCloud.current;
            if (!expected || expected.session !== data.session) return;
            pendingCloud.current = null;
            if (data.status !== "ok") {
                setBusy(null);
                if (data.error !== "access_denied") {
                    toast.error(data.message || (data.error ? `Provider error: ${data.error}` : "Connection was cancelled."));
                }
                return;
            }
            void toast
                .promise(
                    finishCloudOAuth(data.session).then((inbox) => {
                        qc.invalidateQueries({ queryKey: ["emails", "list"] });
                        qc.invalidateQueries({ queryKey: ["cloud-link"] });
                        capture("mailbox_connected", { provider: expected.provider, method: "cloud" });
                        optionsRef.current.onConnected?.(expected.provider, inbox);
                        return inbox;
                    }),
                    {
                        loading: optionsRef.current.messages?.loading ?? "Adding the mailbox…",
                        success: optionsRef.current.messages?.success ?? "Mailbox connected. Warmbly Cloud warms it from now on.",
                        error: (e: AppError) => buildError(e),
                    },
                )
                .catch(onConnectError)
                .finally(() => setBusy(null));
        }
        window.addEventListener("message", onMessage);
        return () => window.removeEventListener("message", onMessage);
    }, [qc, onConnectError]);

    // Only a message from an origin we own whose state matches the one we issued is honoured.
    React.useEffect(() => {
        function onMessage(event: MessageEvent) {
            if (event.origin && !allowedCallbackOrigins().includes(event.origin)) return;
            const data = event.data as OAuthCallbackMessage | undefined;
            if (!data || data.type !== "email_oauth_callback") return;
            // An admin grant sign-in (useAdminGrantPopup) finishes elsewhere.
            if (isAdminConsentState(data.state)) return;

            const expected = pendingState.current;
            if (!expected || expected.state !== data.state) return;
            pendingState.current = null;

            if (data.error || !data.code) {
                setBusy(null);
                if (data.error !== "access_denied") {
                    toast.error(data.error ? `Provider error: ${data.error}` : "Connection was cancelled.");
                }
                return;
            }

            void toast
                .promise(
                    onboardOAuthFinish(data.code, data.state).then((inbox) => {
                        qc.invalidateQueries({ queryKey: ["emails", "list"] });
                        capture("mailbox_connected", { provider: expected.provider, method: "oauth" });
                        optionsRef.current.onConnected?.(expected.provider, inbox);
                        return inbox;
                    }),
                    {
                        loading: optionsRef.current.messages?.loading ?? "Connecting…",
                        success: optionsRef.current.messages?.success ?? "Mailbox connected",
                        error: (e: AppError) => buildError(e),
                    },
                )
                .catch(onConnectError)
                .finally(() => setBusy(null));
        }
        window.addEventListener("message", onMessage);
        return () => window.removeEventListener("message", onMessage);
    }, [qc, onConnectError]);

    const start = React.useCallback(
        async (provider: MailboxOAuthProvider, opts: { loginHint?: string } = {}) => {
            if (busy) return;
            setBusy(provider);
            if (viaCloud) {
                // The cloud's start takes no hint; its consent screen asks for the account.
                try {
                    const { url, session } = await startCloudOAuth(provider);
                    pendingCloud.current = { provider, session };
                    const popup = openCentered(url, `connect-${provider}`);
                    if (!popup) {
                        pendingCloud.current = null;
                        setBusy(null);
                        toast.error("Could not open the authorization window. Please allow popups and try again.");
                    }
                } catch (err) {
                    pendingCloud.current = null;
                    setBusy(null);
                    if (isAllowanceError(err)) {
                        optionsRef.current.onAllowance?.();
                        return;
                    }
                    toast.error(buildError(err as AppError));
                }
                return;
            }
            try {
                const { url, state, admin_consent_url } = await onboardOAuthStart(provider, opts.loginHint);
                if (provider === "outlook" && admin_consent_url) setAdminConsentUrl(admin_consent_url);
                pendingState.current = { provider, state };
                const popup = openCentered(url, `connect-${provider}`);
                if (!popup) {
                    pendingState.current = null;
                    setBusy(null);
                    toast.error("Could not open the authorization window. Please allow popups and try again.");
                }
            } catch (err) {
                pendingState.current = null;
                setBusy(null);
                const e = err as AppError;
                if (e.code === "mailbox_provider_not_configured") {
                    const cb = optionsRef.current.onNotConfigured;
                    if (cb) cb(provider);
                    else
                        toast.error(
                            `${provider === "gmail" ? "Google" : "Microsoft"} sign-in is not configured on this deployment.`,
                        );
                    return;
                }
                if (isAllowanceError(e)) {
                    optionsRef.current.onAllowance?.();
                    return;
                }
                toast.error(buildError(e));
            }
        },
        [busy, viaCloud],
    );

    // Forget any popup still out, e.g. when the dialog that opened it closes.
    const reset = React.useCallback(() => {
        setBusy(null);
        setAdminConsentUrl(null);
        pendingState.current = null;
        pendingCloud.current = null;
    }, []);

    return { busy, start, reset, viaCloud, selfHosted: pool.selfHosted, adminConsentUrl };
}
