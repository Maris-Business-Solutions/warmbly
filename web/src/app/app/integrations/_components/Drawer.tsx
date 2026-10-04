// Side drawer chrome shared by the connection detail and its panels.

"use client";

import React from "react";
import { motion } from "framer-motion";
import { XIcon } from "lucide-react";

import ProviderGlyph from "./ProviderGlyph";

export function Drawer({
    title,
    name,
    provider,
    onClose,
    headerExtra,
    children,
}: {
    title: string;
    name: string;
    provider: string;
    onClose: () => void;
    headerExtra?: React.ReactNode;
    children: React.ReactNode;
}) {
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
                initial={{ x: 28, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
                className="ml-auto h-full w-full sm:w-[480px] sm:max-w-[92vw] bg-white shadow-xl flex flex-col z-10 relative"
            >
                <div className="h-12 px-5 border-b border-slate-200 flex items-center gap-3 shrink-0">
                    <ProviderGlyph provider={provider} name={name} size={7} />
                    <div className="min-w-0 flex-1">
                        <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{title}</div>
                        <div className="text-[12.5px] text-slate-900 font-medium truncate">{name}</div>
                    </div>
                    {headerExtra}
                    <button
                        type="button"
                        onClick={onClose}
                        aria-label="Close"
                        className="h-7 w-7 rounded border border-slate-200 hover:border-slate-300 text-slate-500 hover:text-slate-900 inline-flex items-center justify-center transition-colors"
                    >
                        <XIcon className="w-3.5 h-3.5" />
                    </button>
                </div>
                {children}
            </motion.div>
        </div>
    );
}

export function SectionLabel({ children }: { children: React.ReactNode }) {
    return (
        <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{children}</div>
    );
}
