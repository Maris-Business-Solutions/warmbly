// The link handshake as one reusable card: pitch, then the code with a
// countdown and a live "waiting" pulse, then a linked confirmation. Used by
// Settings > Warmbly Cloud, the onboarding step and the mailboxes-page dialog.

import React from "react";
import { motion } from "framer-motion";
import toast from "react-hot-toast";
import { CheckIcon, CloudIcon, CopyIcon, ExternalLinkIcon, FlameIcon, GlobeIcon, InboxIcon, Loader2Icon, LockIcon, ShieldCheckIcon } from "lucide-react";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import type { CloudLinkPendingConnect } from "@/lib/api/models/app/cloudlink/CloudLink";
import { usePollCloudLinkConnect, useStartCloudLinkConnect } from "@/lib/api/hooks/app/cloudlink/useCloudLink";

export default function CloudLinkCard({
    linked,
    orgName,
    cloudUrl,
    onLinked,
    compact = false,
    autoStart = false,
    minimal = false,
    startSignal = 0,
}: {
    linked: boolean;
    orgName?: string;
    cloudUrl?: string;
    onLinked: (orgName: string) => void;
    /** Tighter layout for dialogs and onboarding (no side illustration). */
    compact?: boolean;
    /** Request a code immediately instead of showing the pitch first. */
    autoStart?: boolean;
    /** Short pitch, no heading or button: the host owns the primary action. */
    minimal?: boolean;
    /** Increment to request a code from outside (the host's primary button). */
    startSignal?: number;
}) {
    const start = useStartCloudLinkConnect();
    const poll = usePollCloudLinkConnect();
    const [pending, setPending] = React.useState<CloudLinkPendingConnect | null>(null);
    const [copied, setCopied] = React.useState(false);
    const started = React.useRef(false);

    const begin = React.useCallback(async () => {
        try {
            const p = await start.mutateAsync(undefined);
            setPending(p);
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    }, [start]);

    React.useEffect(() => {
        if (autoStart && !linked && !pending && !started.current) {
            started.current = true;
            void begin();
        }
    }, [autoStart, linked, pending, begin]);

    React.useEffect(() => {
        if (startSignal > 0 && !linked && !pending) void begin();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [startSignal]);

    // Poll on the cloud's interval while a code is out.
    React.useEffect(() => {
        if (!pending || linked) return;
        let stopped = false;
        const tick = async () => {
            if (stopped) return;
            try {
                const res = await poll.mutateAsync();
                if (res.status === "approved") {
                    onLinked(res.link?.organization_name ?? res.info?.organization.name ?? "");
                    return;
                }
            } catch (e) {
                const err = e as AppError;
                if (err.code === "cloud_link_code_expired" || err.code === "pool_link_denied" || err.code === "pool_link_code_not_found") {
                    toast.error(err.message);
                    setPending(null);
                    return;
                }
            }
            if (!stopped) window.setTimeout(tick, Math.max(2, pending.interval) * 1000);
        };
        const t = window.setTimeout(tick, Math.max(2, pending.interval) * 1000);
        return () => {
            stopped = true;
            window.clearTimeout(t);
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [pending, linked]);

    const copy = async () => {
        if (!pending) return;
        try {
            await navigator.clipboard.writeText(pending.user_code);
            setCopied(true);
            window.setTimeout(() => setCopied(false), 1500);
        } catch {
            toast.error("Could not copy");
        }
    };

    if (linked) {
        return (
            <motion.div initial={{ opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }} className="flex flex-col items-center justify-center text-center gap-3 py-6">
                <motion.span
                    initial={{ scale: 0.6 }}
                    animate={{ scale: 1 }}
                    transition={{ type: "spring", stiffness: 420, damping: 22 }}
                    className="size-12 rounded-full bg-emerald-50 text-emerald-600 inline-flex items-center justify-center"
                >
                    <CheckIcon className="w-6 h-6" />
                </motion.span>
                <div>
                    <p className="text-[14px] font-semibold text-slate-900">Linked{orgName ? ` to ${orgName}` : ""}</p>
                    <p className="text-[12.5px] text-slate-500 mt-0.5">This instance can now warm mailboxes in the Warmbly pool.</p>
                </div>
            </motion.div>
        );
    }

    if (!pending && minimal) {
        return (
            <div className="flex items-start gap-3">
                <span className="size-9 rounded-md bg-sky-600 text-white inline-flex items-center justify-center shrink-0">
                    <FlameIcon className="w-4 h-4" />
                </span>
                <div className="text-[13px] text-slate-600 leading-relaxed">
                    <p className="text-slate-900 font-medium">Warmbly warms your mailboxes for you.</p>
                    <p className="mt-1">
                        Free for 10 mailboxes. Google and Microsoft sign-in without OAuth setup, and domain redirects without proxy setup. Your data stays on
                        this server; only warmup runs in the cloud.
                    </p>
                    {start.isPending && (
                        <p className="mt-2 inline-flex items-center gap-1.5 text-slate-400">
                            <Loader2Icon className="w-3 h-3 animate-spin" /> Getting a code
                        </p>
                    )}
                </div>
            </div>
        );
    }

    if (!pending) {
        return (
            <div className={compact ? "space-y-4" : "grid md:grid-cols-[1fr_260px] gap-6 items-start"}>
                <div className="space-y-4">
                    <div>
                        <h3 className="text-[14px] font-semibold text-slate-900">Warm your mailboxes with the Warmbly pool</h3>
                        <p className="text-[12.5px] text-slate-500 leading-relaxed mt-1">
                            Link this instance to a Warmbly Cloud workspace. Warmbly runs warmup for the mailboxes you choose, using thousands of real
                            mailboxes, while everything else stays on this server.
                        </p>
                    </div>
                    <ul className="space-y-2 text-[12.5px] text-slate-600">
                        <Bullet icon={FlameIcon}>Free for up to 10 mailboxes. Premium, $15 a month, brings better deliverability to every mailbox.</Bullet>
                        <Bullet icon={CloudIcon}>Sign in Google and Microsoft mailboxes through Warmbly's own apps: no OAuth client setup on this server.</Bullet>
                        <Bullet icon={GlobeIcon}>Have the cloud serve your sending domains&apos; website redirects, with no proxy or certificate setup on this server.</Bullet>
                        <Bullet icon={LockIcon}>
                            Only mailbox sign-ins live on the cloud, encrypted, for warmup and sending, plus the domain and website of any redirect you hand it.
                            Campaigns, contacts and inbox never leave here.
                        </Bullet>
                        <Bullet icon={ShieldCheckIcon}>You can remove a mailbox or disconnect at any time; the cloud forgets it immediately.</Bullet>
                    </ul>
                    <button
                        type="button"
                        onClick={() => void begin()}
                        disabled={start.isPending}
                        className="h-8 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12.5px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                    >
                        {start.isPending ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <CloudIcon className="w-3.5 h-3.5" />}
                        Connect to Warmbly Cloud
                    </button>
                    {cloudUrl && <p className="text-[11px] text-slate-400">Connecting to {cloudUrl}</p>}
                </div>
                {!compact && <Illustration />}
            </div>
        );
    }

    return (
        <div className="flex flex-col items-center text-center gap-4 py-2">
            <p className="text-[12.5px] text-slate-500">Enter this code on Warmbly Cloud to approve the link.</p>
            <motion.button
                type="button"
                onClick={copy}
                initial={{ scale: 0.96, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                className="group relative rounded-lg border border-slate-200 bg-slate-50 px-6 py-4 font-mono text-[28px] tracking-[0.28em] text-slate-900 hover:border-sky-300 transition-colors"
                title="Copy code"
                data-ph-mask=""
            >
                {pending.user_code}
                <span className="absolute -top-2 -right-2 size-6 rounded-full bg-white border border-slate-200 text-slate-500 inline-flex items-center justify-center group-hover:text-sky-600">
                    {copied ? <CheckIcon className="w-3 h-3 text-emerald-600" /> : <CopyIcon className="w-3 h-3" />}
                </span>
            </motion.button>
            <a
                href={pending.verification_url}
                target="_blank"
                rel="noreferrer"
                className="h-8 px-3 rounded-md bg-slate-900 hover:bg-slate-800 text-white text-[12.5px] font-medium inline-flex items-center gap-1.5 transition-colors"
            >
                Open Warmbly Cloud
                <ExternalLinkIcon className="w-3.5 h-3.5" />
            </a>
            <div className="inline-flex items-center gap-2 text-[12px] text-slate-500">
                <span className="relative flex size-2">
                    <span className="absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75 animate-ping" />
                    <span className="relative inline-flex size-2 rounded-full bg-sky-500" />
                </span>
                Waiting for approval
                <Countdown until={pending.expires_at} />
            </div>
            <p className="text-[11px] text-slate-400 max-w-sm">
                No Warmbly account yet? The link takes you to sign up first; it is free. This tab keeps waiting.
            </p>
        </div>
    );
}

function Countdown({ until }: { until: Date }) {
    const [left, setLeft] = React.useState(() => Math.max(0, Math.floor((new Date(until).getTime() - Date.now()) / 1000)));
    React.useEffect(() => {
        const t = window.setInterval(() => setLeft(Math.max(0, Math.floor((new Date(until).getTime() - Date.now()) / 1000))), 1000);
        return () => window.clearInterval(t);
    }, [until]);
    const m = Math.floor(left / 60);
    const s = left % 60;
    return (
        <span className="tabular-nums text-slate-400">
            · code expires in {m}:{s.toString().padStart(2, "0")}
        </span>
    );
}

function Bullet({ icon: Icon, children }: { icon: React.ComponentType<{ className?: string }>; children: React.ReactNode }) {
    return (
        <li className="flex items-start gap-2">
            <Icon className="w-3.5 h-3.5 mt-0.5 text-sky-600 shrink-0" />
            <span className="leading-relaxed">{children}</span>
        </li>
    );
}

// A small animated diagram: this server, the cloud, and mail flowing between.
function Illustration() {
    return (
        <div className="rounded-lg border border-slate-200 bg-gradient-to-b from-sky-50/60 to-white p-4 h-[170px] relative overflow-hidden">
            <div className="absolute left-5 top-1/2 -translate-y-1/2 flex flex-col items-center gap-1.5">
                <span className="size-10 rounded-md bg-white border border-slate-200 inline-flex items-center justify-center text-slate-700">
                    <InboxIcon className="w-4 h-4" />
                </span>
                <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">You</span>
            </div>
            <div className="absolute right-5 top-1/2 -translate-y-1/2 flex flex-col items-center gap-1.5">
                <span className="size-10 rounded-md bg-sky-600 inline-flex items-center justify-center text-white">
                    <CloudIcon className="w-4 h-4" />
                </span>
                <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Pool</span>
            </div>
            <div className="absolute left-[72px] right-[72px] top-1/2 -translate-y-1/2 h-px bg-slate-200" />
            {[0, 1, 2].map((i) => (
                <motion.span
                    key={i}
                    className="absolute top-1/2 -translate-y-1/2 size-1.5 rounded-full bg-sky-500"
                    initial={{ left: 72, opacity: 0 }}
                    animate={{ left: ["72px", "calc(100% - 78px)"], opacity: [0, 1, 1, 0] }}
                    transition={{ duration: 2.4, delay: i * 0.8, repeat: Infinity, ease: "easeInOut" }}
                />
            ))}
        </div>
    );
}
