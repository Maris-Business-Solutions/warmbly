// Modal wrapper around CloudLinkCard for surfaces outside Settings (the
// mailboxes page, sending domains). Same overlay anatomy as NewCampaignDialog.
// Portaled and layered above drawers, which a transformed parent would trap.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { LockIcon, XIcon } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import CloudLinkCard from "./CloudLinkCard";
import { CLOUD_LINK_KEY } from "@/lib/api/hooks/app/cloudlink/useCloudLink";
import { usePermission } from "@/hooks/usePermission";

export default function CloudConnectDialog({
    open,
    onClose,
    cloudUrl,
    intro,
    autoStart = false,
    doneLabel = "Pick mailboxes",
    onDone,
}: {
    open: boolean;
    onClose: () => void;
    cloudUrl?: string;
    /** Why the link is asked for here, shown above the code in place of the warmup pitch. */
    intro?: React.ReactNode;
    /** Ask for a code straight away. */
    autoStart?: boolean;
    doneLabel?: string;
    /** What the button after linking does; closing is the default. */
    onDone?: () => void;
}) {
    const qc = useQueryClient();
    const canLink = usePermission("MANAGE_SETTINGS");
    const [linked, setLinked] = React.useState(false);
    const [orgName, setOrgName] = React.useState("");

    React.useEffect(() => {
        if (!open) {
            setLinked(false);
            setOrgName("");
        }
    }, [open]);

    React.useEffect(() => {
        if (!open) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "Escape") return;
            if (document.querySelector("[data-floating]:not([data-cloud-connect]), [role='alertdialog']")) return;
            e.preventDefault();
            onClose();
        };
        document.addEventListener("keydown", onKey);
        return () => document.removeEventListener("keydown", onKey);
    }, [open, onClose]);

    return createPortal(
        <AnimatePresence>
            {open && (
                <motion.div
                    key="overlay"
                    data-floating
                    data-cloud-connect
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.15 }}
                    onMouseDown={(e) => {
                        e.stopPropagation();
                        onClose();
                    }}
                    className="fixed inset-0 z-[120] flex items-center justify-center bg-slate-900/30 backdrop-blur-[2px] px-4"
                >
                    <motion.div
                        key="card"
                        role="dialog"
                        aria-modal="true"
                        aria-label="Connect to Warmbly Cloud"
                        initial={{ y: 8, opacity: 0, scale: 0.985 }}
                        animate={{ y: 0, opacity: 1, scale: 1 }}
                        exit={{ y: 8, opacity: 0, scale: 0.985 }}
                        transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                        onMouseDown={(e) => e.stopPropagation()}
                        className="w-full max-w-[520px] rounded-lg bg-white border border-slate-200 shadow-[0_24px_48px_-12px_rgba(15,23,42,0.18)] overflow-hidden"
                    >
                        <div className="h-11 px-4 flex items-center justify-between border-b border-slate-200/70">
                            <span className="text-[12.5px] font-semibold text-slate-900">Warmbly Cloud</span>
                            <button type="button" onClick={onClose} aria-label="Close" className="size-7 rounded-md inline-flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-100">
                                <XIcon className="w-4 h-4" />
                            </button>
                        </div>
                        <div className="px-5 py-5 space-y-4">
                            {intro && !linked && <div className="text-[12px] text-slate-600 leading-relaxed">{intro}</div>}
                            {canLink ? (
                                <CloudLinkCard
                                    compact
                                    autoStart={autoStart}
                                    linked={linked}
                                    orgName={orgName}
                                    cloudUrl={cloudUrl}
                                    onLinked={(name) => {
                                        setOrgName(name);
                                        setLinked(true);
                                        void qc.invalidateQueries({ queryKey: CLOUD_LINK_KEY });
                                    }}
                                />
                            ) : (
                                <p className="rounded-md border border-slate-200 bg-slate-50/60 px-3 py-2.5 text-[12px] text-slate-600 leading-relaxed flex items-start gap-2">
                                    <LockIcon className="w-3.5 h-3.5 text-slate-400 mt-0.5 shrink-0" />
                                    <span>Connecting this instance to Warmbly Cloud is a settings change. Ask a teammate who manages settings to connect it.</span>
                                </p>
                            )}
                        </div>
                        {linked && (
                            <div className="px-5 py-3 border-t border-slate-200/70 flex justify-end">
                                <button
                                    type="button"
                                    onClick={onDone ?? onClose}
                                    className="h-7 px-2.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium transition-colors"
                                >
                                    {doneLabel}
                                </button>
                            </div>
                        )}
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>,
        document.body,
    );
}
