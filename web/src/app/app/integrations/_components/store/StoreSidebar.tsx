// The store's rail, drawn like the Settings rail: search, then grouped links.

import React from "react";
import { Link } from "@tanstack/react-router";
import { motion } from "framer-motion";

import { cn } from "@/lib/utils";

export interface NavEntry {
    key: string;
    label: string;
    to: string;
    icon: React.ElementType;
    count?: number;
}

export default function StoreSidebar({
    search,
    groups,
    active,
}: {
    search: React.ReactNode;
    groups: { label?: string; items: NavEntry[] }[];
    active: string;
}) {
    return (
        <aside className="hidden md:block w-[236px] shrink-0 self-stretch border-r border-slate-200/70">
            <div className="sticky top-12 max-h-[calc(100dvh-3rem)] overflow-y-auto px-2.5 py-3">
                <div className="px-0.5 mb-3">{search}</div>
                {groups.map((g, gi) =>
                    g.items.length === 0 ? null : (
                        <nav key={gi} className="mb-1" aria-label={g.label ?? "Integrations"}>
                            {g.label && (
                                <div className="px-2 mt-3 mb-1 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">
                                    {g.label}
                                </div>
                            )}
                            {g.items.map((it) => {
                                const on = active === it.key;
                                return (
                                    <Link
                                        key={it.key}
                                        to={it.to}
                                        activeOptions={{ exact: true, includeSearch: false }}
                                        aria-current={on ? "page" : undefined}
                                        className={cn(
                                            "group relative w-full flex items-center gap-2.5 px-2.5 h-8 rounded-md text-[12.5px] transition-colors",
                                            on ? "text-slate-900 font-medium" : "text-slate-600 hover:text-slate-900 hover:bg-slate-200/40",
                                        )}
                                    >
                                        {on && (
                                            <motion.span
                                                layoutId="integrations-active-pill"
                                                className="absolute inset-0 rounded-md bg-slate-200/70"
                                                transition={{ type: "spring", stiffness: 520, damping: 42 }}
                                            />
                                        )}
                                        <it.icon
                                            className={cn(
                                                "relative z-10 w-[14px] h-[14px] shrink-0",
                                                on ? "text-slate-700" : "text-slate-400 group-hover:text-slate-600",
                                            )}
                                        />
                                        <span className="relative z-10 flex-1 truncate">{it.label}</span>
                                        {!!it.count && <span className="relative z-10 text-[11px] text-slate-400 tabular-nums">{it.count}</span>}
                                    </Link>
                                );
                            })}
                        </nav>
                    ),
                )}
            </div>
        </aside>
    );
}
