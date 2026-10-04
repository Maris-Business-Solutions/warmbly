// Landing page for the Google/Microsoft popup that Warmbly Cloud ran on a
// linked instance's behalf. Hands the session back to the opener (the Add
// account dialog) and closes; without an opener it explains what happened.

import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckIcon, XIcon } from "lucide-react";
import { Logo } from "@/components/svg";

export interface CloudOAuthDoneMessage {
    type: "cloud_oauth_callback";
    session: string;
    status: "ok" | "error";
    error?: string;
    message?: string;
}

export default function CloudOAuthDonePage() {
    const params = new URLSearchParams(window.location.search);
    const session = params.get("session") ?? "";
    const status = params.get("status") === "ok" ? "ok" : "error";
    const error = params.get("error") ?? "";
    const message = params.get("message") ?? "";
    const [delivered, setDelivered] = React.useState(false);

    React.useEffect(() => {
        const payload: CloudOAuthDoneMessage = { type: "cloud_oauth_callback", session, status, error, message };
        let ok = false;
        try {
            if (window.opener) {
                window.opener.postMessage(payload, window.location.origin);
                ok = true;
            }
        } catch {
            /* no opener */
        }
        setDelivered(ok);
        if (ok) window.setTimeout(() => window.close(), 400);
    }, [session, status, error, message]);

    return (
        <div className="min-h-dvh flex items-center justify-center bg-slate-50 px-4">
            <div className="w-full max-w-sm rounded-xl border border-slate-200 bg-white p-6 text-center shadow-sm">
                <Logo className="w-7 mx-auto text-slate-900" />
                <span className={`mt-4 mx-auto size-10 rounded-full inline-flex items-center justify-center ${status === "ok" ? "bg-emerald-50 text-emerald-600" : "bg-rose-50 text-rose-600"}`}>
                    {status === "ok" ? <CheckIcon className="w-5 h-5" /> : <XIcon className="w-5 h-5" />}
                </span>
                <p className="mt-3 text-[14px] font-semibold text-slate-900">{status === "ok" ? "Mailbox signed in" : "Sign-in did not complete"}</p>
                <p className="mt-1 text-[12.5px] text-slate-500 leading-relaxed">
                    {delivered
                        ? "This window closes on its own."
                        : status === "ok"
                          ? "Go back to the Warmbly tab; the mailbox is being added there."
                          : message || error || "Try again from Add account."}
                </p>
                {!delivered && (
                    <Link to="/app/emails" className="mt-4 inline-flex h-8 px-3 items-center rounded-md bg-slate-900 text-white text-[12.5px] font-medium">
                        Back to mailboxes
                    </Link>
                )}
            </div>
        </div>
    );
}
