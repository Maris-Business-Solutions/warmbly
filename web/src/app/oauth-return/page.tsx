// Where a sign-in window without an opener lands: the API's callback page
// sends the result here so it can reach the dashboard tab that is waiting.

import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckIcon, XIcon } from "lucide-react";
import { Logo } from "@/components/svg";
import { oauthReturnMessage, relayOAuthReturn } from "@/lib/oauthReturn";

export default function OAuthReturnPage() {
    const [message] = React.useState(() => oauthReturnMessage(window.location.hash));
    const [delivered, setDelivered] = React.useState(false);

    React.useEffect(() => {
        // The fragment carries a single-use code; keep it out of history.
        window.history.replaceState(null, "", window.location.pathname);
        if (!message || !relayOAuthReturn(message)) return;
        setDelivered(true);
        const t = window.setTimeout(() => window.close(), 400);
        return () => window.clearTimeout(t);
    }, [message]);

    const ok = Boolean(message && "code" in message && message.code && !message.error);
    return (
        <div className="min-h-dvh flex items-center justify-center bg-slate-50 px-4">
            <div className="w-full max-w-sm rounded-xl border border-slate-200 bg-white p-6 text-center shadow-sm">
                <Logo className="w-7 mx-auto text-slate-900" />
                <span className={`mt-4 mx-auto size-10 rounded-full inline-flex items-center justify-center ${ok ? "bg-emerald-50 text-emerald-600" : "bg-rose-50 text-rose-600"}`}>
                    {ok ? <CheckIcon className="w-5 h-5" /> : <XIcon className="w-5 h-5" />}
                </span>
                <p className="mt-3 text-[14px] font-semibold text-slate-900">{ok ? "Signed in" : "Sign-in did not complete"}</p>
                <p className="mt-1 text-[12.5px] text-slate-500 leading-relaxed">
                    {delivered
                        ? "Finishing in your Warmbly tab. This window closes on its own; if it stays open, close it and go back to that tab."
                        : "This window could not reach your Warmbly tab. Close it and try again from there."}
                </p>
                {!delivered && (
                    <Link to="/app" className="mt-4 inline-flex h-8 px-3 items-center rounded-md bg-slate-900 text-white text-[12.5px] font-medium">
                        Back to Warmbly
                    </Link>
                )}
            </div>
        </div>
    );
}
