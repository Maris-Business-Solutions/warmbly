// Detail drawer for a community app. It opens from a directory card and from a
// shared link (/app/integrations/apps/:slug), which is the only way to reach an
// app that is not verified yet, so the unverified warning leads the drawer.

"use client";

import React from "react";
import { motion } from "framer-motion";
import { useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import toast from "react-hot-toast";
import {
    BadgeCheckIcon,
    CheckIcon,
    ExternalLinkIcon,
    LinkIcon,
    Loader2Icon,
    ShieldAlertIcon,
    ShieldCheckIcon,
    XIcon,
} from "lucide-react";

import { useConfirm } from "@/hooks/context/confirm";
import { useCommunityApp } from "@/lib/api/hooks/app/integrations/useCommunityApps";
import { useAuthorizedApps, useRevokeAuthorizedApp } from "@/lib/api/hooks/app/oauth/useAuthorizedApps";
import {
    communityAppPath,
    LISTING_CATEGORY_LABELS,
    type CommunityApp,
} from "@/lib/api/models/app/integrations/Community";
import { cn } from "@/lib/utils";

import CommunityLogo from "./CommunityLogo";
import { primaryBtn, SectionLabel } from "./ConnectDrawer";

function hostOf(url: string): string {
    try {
        return new URL(url).host;
    } catch {
        return url;
    }
}

export default function CommunityAppDrawer({
    slug,
    preview,
    onClose,
}: {
    slug: string;
    /** The card's copy, shown while the full listing loads. */
    preview?: CommunityApp;
    onClose: () => void;
}) {
    const query = useCommunityApp(slug);
    const app = query.data ?? preview;

    React.useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if (e.key !== "Escape") return;
            if (document.querySelector('[data-floating], [role="alertdialog"]')) return;
            onClose();
        }
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, [onClose]);

    function copyLink() {
        void navigator.clipboard.writeText(`${window.location.origin}${communityAppPath(slug)}`);
        toast.success("Link copied");
    }

    return (
        <div className="fixed inset-0 z-40 flex">
            <motion.button
                type="button"
                aria-label="Close"
                onClick={onClose}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 0.18 }}
                className="absolute inset-0 bg-slate-900/30 backdrop-blur-[2px]"
            />
            <motion.div
                role="dialog"
                aria-modal="true"
                aria-label={app?.name ?? "Community app"}
                onMouseDown={(e) => e.stopPropagation()}
                initial={{ x: 28, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
                className="ml-auto h-full w-full sm:w-[480px] sm:max-w-[92vw] bg-white shadow-xl flex flex-col z-10 relative"
            >
                <div className="h-12 px-5 border-b border-slate-200 flex items-center gap-3 shrink-0">
                    {app ? <CommunityLogo name={app.name} url={app.logo_url} size={7} /> : <div className="w-7 h-7 rounded-md bg-slate-100" />}
                    <div className="min-w-0 flex-1">
                        <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Community app</div>
                        <div className="text-[12.5px] text-slate-900 font-medium truncate">{app?.name ?? "Loading"}</div>
                    </div>
                    <button
                        type="button"
                        onClick={copyLink}
                        aria-label="Copy link"
                        title="Copy link"
                        className="h-7 w-7 rounded border border-slate-200 hover:border-slate-300 text-slate-500 hover:text-slate-900 inline-flex items-center justify-center transition-colors"
                    >
                        <LinkIcon className="w-3.5 h-3.5" />
                    </button>
                    <button
                        type="button"
                        onClick={onClose}
                        aria-label="Close"
                        className="h-7 w-7 rounded border border-slate-200 hover:border-slate-300 text-slate-500 hover:text-slate-900 inline-flex items-center justify-center transition-colors"
                    >
                        <XIcon className="w-3.5 h-3.5" />
                    </button>
                </div>

                {query.isError && !preview ? (
                    <div className="flex-1 px-5 py-16 text-center">
                        <p className="text-[12.5px] text-slate-700 font-medium mb-1">This app isn’t available</p>
                        <p className="text-[11.5px] text-slate-400 max-w-[36ch] mx-auto leading-relaxed">
                            Its developer may have unpublished it, or the link is wrong. Ask them for a new link.
                        </p>
                    </div>
                ) : !app ? (
                    <div className="flex-1 flex items-center justify-center">
                        <Loader2Icon className="w-4 h-4 animate-spin text-slate-400" />
                    </div>
                ) : (
                    <AppBody app={app} onClose={onClose} />
                )}
            </motion.div>
        </div>
    );
}

function AppBody({ app, onClose }: { app: CommunityApp; onClose: () => void }) {
    const authorized = useAuthorizedApps();
    const revoke = useRevokeAuthorizedApp();
    const confirm = useConfirm();
    const qc = useQueryClient();
    const mine = (authorized.data?.authorized_apps ?? []).some((a) => a.application_id === app.application_id);
    const verified = app.verification === "verified";
    const reads = app.permissions.filter((p) => p.category === "read");
    const writes = app.permissions.filter((p) => p.category !== "read");

    function install() {
        window.open(app.install_url, "_blank", "noopener,noreferrer");
    }

    return (
        <>
            <div className="flex-1 overflow-y-auto px-5 py-5 space-y-5">
                {verified ? (
                    <div className="flex items-start gap-2 rounded-md border border-emerald-200 bg-emerald-50/60 px-3 py-2">
                        <BadgeCheckIcon className="w-3.5 h-3.5 text-emerald-600 mt-0.5 shrink-0" />
                        <p className="text-[12px] text-emerald-800 leading-relaxed">
                            Verified. This listing was reviewed before it was added to the directory.
                        </p>
                    </div>
                ) : (
                    <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5">
                        <ShieldAlertIcon className="w-3.5 h-3.5 text-amber-600 mt-0.5 shrink-0" />
                        <div className="space-y-1">
                            <p className="text-[12px] font-medium text-amber-900">Unverified app</p>
                            <p className="text-[11.5px] text-amber-800 leading-relaxed">
                                Nobody has reviewed this app yet, so it isn’t listed in the directory and you can only open it
                                from a link. Install it only if you know and trust {app.developer || "its developer"}.
                            </p>
                        </div>
                    </div>
                )}

                <div>
                    <p className="text-[13px] text-slate-800 leading-relaxed">{app.tagline}</p>
                    {app.description && (
                        <p className="mt-2 text-[12.5px] text-slate-600 leading-relaxed whitespace-pre-line break-words">
                            {app.description}
                        </p>
                    )}
                </div>

                <dl className="grid grid-cols-2 gap-px rounded-md border border-slate-200 bg-slate-200 overflow-hidden">
                    <Fact label="Developer" value={app.developer || "Unknown"} />
                    <Fact label="Category" value={LISTING_CATEGORY_LABELS[app.category] ?? app.category} />
                    <Fact
                        label="Used by"
                        value={app.installs === 1 ? "1 workspace" : `${app.installs.toLocaleString()} workspaces`}
                    />
                    <Fact
                        label="Website"
                        value={
                            app.website_url ? (
                                <a
                                    href={app.website_url}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="text-sky-700 hover:underline truncate block"
                                >
                                    {hostOf(app.website_url)}
                                </a>
                            ) : (
                                "None"
                            )
                        }
                    />
                </dl>

                <div className="space-y-2">
                    <SectionLabel>What it can ask for</SectionLabel>
                    {app.permissions.length === 0 ? (
                        <p className="text-[12px] text-slate-500">No permissions.</p>
                    ) : (
                        <div className="rounded-md border border-slate-200 divide-y divide-slate-200">
                            {[...writes, ...reads].map((p) => (
                                <div key={p.name} className="flex items-start gap-2 px-2.5 py-1.5">
                                    <ShieldCheckIcon
                                        className={cn(
                                            "w-3 h-3 mt-0.5 shrink-0",
                                            p.category === "read" ? "text-slate-400" : "text-amber-500",
                                        )}
                                    />
                                    <div className="min-w-0">
                                        <div className="text-[11.5px] font-medium text-slate-700">{p.description}</div>
                                        <code className="text-[10px] text-slate-400 font-mono">{p.name.toLowerCase()}</code>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                    <p className="text-[11px] text-slate-500 leading-relaxed">
                        Install opens {hostOf(app.install_url)}. The app then sends you back to Warmbly, where you approve
                        what it gets. You can remove its access at any time.
                    </p>
                </div>

                {(app.support_url || app.privacy_url) && (
                    <div className="flex flex-wrap gap-3">
                        {app.support_url && <ExtLink href={app.support_url}>Support</ExtLink>}
                        {app.privacy_url && <ExtLink href={app.privacy_url}>Privacy policy</ExtLink>}
                    </div>
                )}
            </div>

            <div className="mt-auto border-t border-slate-200 px-5 py-3 flex items-center gap-2 shrink-0">
                {app.installed && (
                    <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded text-[9.5px] uppercase tracking-[0.08em] font-medium bg-emerald-50 text-emerald-700">
                        <CheckIcon className="w-3 h-3" />
                        Installed
                    </span>
                )}
                <div className="ml-auto flex items-center gap-2">
                    {mine ? (
                        <button
                            type="button"
                            onClick={() =>
                                confirm.show(`Remove ${app.name}’s access? Its tokens stop working right away.`, async () => {
                                    await revoke.mutateAsync(app.application_id);
                                    void qc.invalidateQueries({ queryKey: ["integrations", "community"] });
                                    toast.success(`${app.name} disconnected`);
                                    onClose();
                                })
                            }
                            className="h-7 px-3 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:bg-rose-50 hover:text-rose-600 hover:border-rose-200 transition-colors"
                        >
                            Disconnect
                        </button>
                    ) : app.installed ? (
                        <Link
                            to="/app/settings/oauth-apps"
                            className="h-7 px-3 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-slate-300 inline-flex items-center"
                        >
                            Manage access
                        </Link>
                    ) : null}
                    <button type="button" onClick={install} className={primaryBtn}>
                        <ExternalLinkIcon className="w-3.5 h-3.5" />
                        {app.installed ? "Open app" : "Install"}
                    </button>
                </div>
            </div>
        </>
    );
}

function Fact({ label, value }: { label: string; value: React.ReactNode }) {
    return (
        <div className="bg-white px-3 py-2 min-w-0">
            <dt className="text-[10px] uppercase tracking-[0.14em] text-slate-400">{label}</dt>
            <dd className="mt-0.5 text-[12.5px] text-slate-800 truncate">{value}</dd>
        </div>
    );
}

function ExtLink({ href, children }: { href: string; children: React.ReactNode }) {
    return (
        <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-[11.5px] text-slate-500 hover:text-sky-700 inline-flex items-center gap-1 underline decoration-dotted underline-offset-2"
        >
            <ExternalLinkIcon className="w-3 h-3" />
            {children}
        </a>
    );
}
