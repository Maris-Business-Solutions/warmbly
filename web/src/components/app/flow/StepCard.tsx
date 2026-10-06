// StepCard is the one card every flow builder draws: an icon tile, a kicker
// naming the kind of step, a title, and a one-line summary of its setup. The
// card has a fixed size because the tree layout places it without measuring.

import React from "react";
import { MoreHorizontalIcon } from "lucide-react";
import { PopoverMenu, PopoverMenuContent, PopoverMenuItem, PopoverMenuSeparator, PopoverMenuTrigger } from "@/components/ui/popover-menu";
import { cn } from "@/lib/utils";

export const STEP_W = 280;
export const STEP_H = 78;
export const TERMINAL_W = 140;
export const TERMINAL_H = 34;

export type StepTone = "sky" | "amber" | "purple" | "violet" | "emerald" | "rose" | "indigo" | "fuchsia" | "orange" | "slate";

const TILE: Record<StepTone, string> = {
    sky: "bg-sky-50 text-sky-600 ring-sky-100",
    amber: "bg-amber-50 text-amber-600 ring-amber-100",
    purple: "bg-purple-50 text-purple-600 ring-purple-100",
    violet: "bg-violet-50 text-violet-600 ring-violet-100",
    emerald: "bg-emerald-50 text-emerald-600 ring-emerald-100",
    rose: "bg-rose-50 text-rose-600 ring-rose-100",
    indigo: "bg-indigo-50 text-indigo-600 ring-indigo-100",
    fuchsia: "bg-fuchsia-50 text-fuchsia-600 ring-fuchsia-100",
    orange: "bg-orange-50 text-orange-600 ring-orange-100",
    slate: "bg-slate-100 text-slate-500 ring-slate-200",
};

export function StepTile({ tone, children, size = "md" }: { tone: StepTone; children: React.ReactNode; size?: "sm" | "md" }) {
    return (
        <span
            className={cn(
                "inline-flex shrink-0 items-center justify-center rounded-lg ring-1 ring-inset [&_svg]:size-4",
                size === "md" ? "size-9" : "size-7 rounded-md [&_svg]:size-3.5",
                TILE[tone],
            )}
        >
            {children}
        </span>
    );
}

export type BadgeTone = "warn" | "ok" | "error" | "muted" | "info";

const BADGE: Record<BadgeTone, string> = {
    warn: "bg-amber-50 text-amber-700 ring-amber-200",
    ok: "bg-emerald-50 text-emerald-700 ring-emerald-200",
    error: "bg-rose-50 text-rose-700 ring-rose-200",
    muted: "bg-slate-50 text-slate-500 ring-slate-200",
    info: "bg-sky-50 text-sky-700 ring-sky-200",
};

export function StepBadge({ tone, children, title }: { tone: BadgeTone; children: React.ReactNode; title?: string }) {
    return (
        <span
            title={title}
            className={cn("inline-flex h-4 shrink-0 items-center gap-1 rounded px-1.5 text-[10px] font-medium ring-1 ring-inset [&_svg]:size-2.5", BADGE[tone])}
        >
            {children}
        </span>
    );
}

export interface StepMenuItem {
    label: string;
    icon?: React.ReactNode;
    onSelect: () => void;
    danger?: boolean;
    disabled?: boolean;
}

export function StepCard({
    icon,
    tile,
    tone,
    kicker,
    title,
    summary,
    badge,
    selected,
    detached,
    menu,
}: {
    icon: React.ReactNode;
    // Replaces the icon tile (a brand logo).
    tile?: React.ReactNode;
    tone: StepTone;
    kicker: string;
    title: string;
    summary?: string;
    badge?: React.ReactNode;
    selected?: boolean;
    detached?: boolean;
    menu?: StepMenuItem[];
}) {
    const items = menu ?? [];
    const plain = items.filter((m) => !m.danger);
    const danger = items.filter((m) => m.danger);
    return (
        <div
            className={cn(
                "group relative flex h-full w-full cursor-pointer items-start gap-2.5 rounded-xl border bg-white px-3 py-2.5 shadow-[0_1px_2px_rgba(15,23,42,0.05)] transition-[border-color,box-shadow] duration-150",
                selected ? "border-sky-400 ring-[3px] ring-sky-100" : "border-slate-200 hover:border-slate-300 hover:shadow-[0_2px_6px_rgba(15,23,42,0.07)]",
                detached && !selected && "border-dashed",
            )}
        >
            {tile ?? <StepTile tone={tone}>{icon}</StepTile>}
            <div className="min-w-0 flex-1">
                <div className="flex h-4 items-center gap-1.5">
                    <span className="truncate text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">{kicker}</span>
                    <span className="ml-auto flex items-center gap-1 pr-5">{badge}</span>
                </div>
                <div className="mt-0.5 truncate text-[13px] font-semibold leading-5 text-slate-900">{title}</div>
                <div className="truncate text-[11.5px] leading-4 text-slate-500">{summary || " "}</div>
            </div>
            {items.length > 0 && (
                <div className="nodrag nopan absolute right-1.5 top-1.5" onClick={(e) => e.stopPropagation()}>
                    <PopoverMenu align="end">
                        <PopoverMenuTrigger asChild>
                            <button
                                type="button"
                                aria-label={`${title} options`}
                                className={cn(
                                    "inline-flex size-6 items-center justify-center rounded-md text-slate-400 transition-[opacity,colors] hover:bg-slate-100 hover:text-slate-700",
                                    selected ? "opacity-100" : "opacity-100 md:opacity-0 md:group-hover:opacity-100 data-[state=open]:opacity-100",
                                )}
                            >
                                <MoreHorizontalIcon className="size-3.5" />
                            </button>
                        </PopoverMenuTrigger>
                        <PopoverMenuContent minWidth={180}>
                            {plain.map((m) => (
                                <PopoverMenuItem key={m.label} icon={m.icon} onSelect={m.onSelect} disabled={m.disabled}>
                                    {m.label}
                                </PopoverMenuItem>
                            ))}
                            {plain.length > 0 && danger.length > 0 && <PopoverMenuSeparator />}
                            {danger.map((m) => (
                                <PopoverMenuItem key={m.label} icon={m.icon} onSelect={m.onSelect} disabled={m.disabled} danger>
                                    {m.label}
                                </PopoverMenuItem>
                            ))}
                        </PopoverMenuContent>
                    </PopoverMenu>
                </div>
            )}
        </div>
    );
}

// A small rounded terminal for the end of a path (an explicit stop).
export function TerminalCard({ icon, label, selected, onRemove }: { icon: React.ReactNode; label: string; selected?: boolean; onRemove?: () => void }) {
    return (
        <div
            className={cn(
                "group flex h-full w-full cursor-pointer items-center justify-center gap-1.5 rounded-full border bg-white px-3 text-[12px] font-medium text-slate-600 shadow-[0_1px_2px_rgba(15,23,42,0.05)]",
                selected ? "border-sky-400 ring-[3px] ring-sky-100" : "border-slate-200 hover:border-slate-300",
            )}
        >
            <span className="text-rose-500 [&_svg]:size-3.5">{icon}</span>
            {label}
            {onRemove && (
                <button
                    type="button"
                    aria-label={`Remove ${label.toLowerCase()}`}
                    onClick={(e) => {
                        e.stopPropagation();
                        onRemove();
                    }}
                    className="nodrag nopan -mr-1 ml-0.5 inline-flex size-5 items-center justify-center rounded-full text-slate-300 opacity-100 transition-opacity hover:bg-rose-50 hover:text-rose-600 md:opacity-0 md:group-hover:opacity-100"
                >
                    ×
                </button>
            )}
        </div>
    );
}
