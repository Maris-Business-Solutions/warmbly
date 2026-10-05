// The popup an integration is managed in once connected: one centered card
// with the provider's mark, its sections and a footer for the lasting actions.

"use client";

import React from "react";
import { motion } from "framer-motion";
import { XIcon } from "lucide-react";

import { cn } from "@/lib/utils";

import ProviderGlyph from "./ProviderGlyph";

export function IntegrationDialog({
    name,
    provider,
    subtitle,
    badge,
    onClose,
    headerExtra,
    tabs,
    footer,
    children,
}: {
    name: string;
    provider: string;
    subtitle?: React.ReactNode;
    badge?: React.ReactNode;
    onClose: () => void;
    headerExtra?: React.ReactNode;
    tabs?: React.ReactNode;
    footer?: React.ReactNode;
    children: React.ReactNode;
}) {
    React.useEffect(() => {
        const onKey = (ev: KeyboardEvent) => {
            if (ev.key !== "Escape") return;
            // An open dropdown, a nested dialog or the confirm owns this Escape.
            if (document.querySelector("[data-floating], [role='alertdialog'], [data-nested-dialog]")) return;
            ev.preventDefault();
            onClose();
        };
        document.addEventListener("keydown", onKey);
        return () => document.removeEventListener("keydown", onKey);
    }, [onClose]);

    return (
        <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            onMouseDown={onClose}
            className="fixed inset-0 z-[100] flex items-center justify-center bg-slate-900/30 backdrop-blur-[2px] px-2 sm:px-4"
        >
            <motion.div
                role="dialog"
                aria-modal="true"
                aria-label={name}
                initial={{ y: 8, opacity: 0, scale: 0.985 }}
                animate={{ y: 0, opacity: 1, scale: 1 }}
                exit={{ y: 8, opacity: 0, scale: 0.985 }}
                transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                onMouseDown={(ev) => ev.stopPropagation()}
                className="relative w-full max-w-[620px] h-[min(90dvh,760px)] rounded-lg bg-white border border-slate-200 shadow-[0_24px_48px_-12px_rgba(15,23,42,0.18),0_8px_16px_-8px_rgba(15,23,42,0.1)] overflow-hidden flex flex-col"
            >
                <header className="px-5 pt-4 pb-3 flex items-start gap-3 shrink-0">
                    <ProviderGlyph provider={provider} name={name} size={9} />
                    <div className="min-w-0 flex-1 pt-0.5">
                        <div className="flex items-center gap-2 min-w-0">
                            <h2 className="text-[15px] font-semibold text-slate-900 tracking-tight truncate">{name}</h2>
                            {badge}
                        </div>
                        {subtitle && <div className="text-[12px] text-slate-500 truncate mt-0.5">{subtitle}</div>}
                    </div>
                    {headerExtra}
                    <button
                        type="button"
                        onClick={onClose}
                        aria-label="Close"
                        className="h-7 w-7 -mr-1 rounded-md text-slate-400 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center justify-center transition-colors shrink-0"
                    >
                        <XIcon className="w-3.5 h-3.5" />
                    </button>
                </header>
                {tabs}
                <div className={cn("flex-1 min-h-0 overflow-y-auto", !tabs && "border-t border-slate-200")}>{children}</div>
                {footer && (
                    <footer className="px-4 h-12 border-t border-slate-200 flex items-center gap-1.5 shrink-0 bg-slate-50/40">
                        {footer}
                    </footer>
                )}
            </motion.div>
        </motion.div>
    );
}

// A section's heading: what it is in a few words, and optionally why it
// matters in a sentence.
export function SectionTitle({ children, description }: { children: React.ReactNode; description?: React.ReactNode }) {
    return (
        <div>
            <h3 className="text-[12.5px] font-semibold text-slate-900">{children}</h3>
            {description && <p className="text-[11.5px] text-slate-500 leading-relaxed mt-0.5">{description}</p>}
        </div>
    );
}
