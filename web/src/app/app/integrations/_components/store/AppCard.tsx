// An app in the store, as a card (grid) or a row (list): icon, name, maker,
// what it does, and one action. The card opens the app's page; the button acts
// on it in place.

import React from "react";
import { BadgeCheckIcon } from "lucide-react";

import type { IntegrationConnection } from "@/lib/api/models/app/integrations/Integration";
import { cn } from "@/lib/utils";

import CommunityLogo from "../CommunityLogo";
import ProviderGlyph from "../ProviderGlyph";
import { categoryLabel, developerLabel, highlightParts, isUsable, type StoreItem } from "./model";

// Text with the search's words marked.
export function Highlight({ text, q }: { text: string; q?: string }) {
    if (!q?.trim()) return <>{text}</>;
    return (
        <>
            {highlightParts(text, q).map((p, i) =>
                p.hit ? (
                    <mark key={i} className="rounded-sm bg-amber-100 text-inherit">
                        {p.text}
                    </mark>
                ) : (
                    <React.Fragment key={i}>{p.text}</React.Fragment>
                ),
            )}
        </>
    );
}

export function ItemLogo({ item, size }: { item: StoreItem; size: 9 | 10 | 12 }) {
    return item.kind === "builtin" ? (
        <ProviderGlyph provider={item.entry.provider} name={item.name} size={size} />
    ) : (
        <CommunityLogo name={item.name} url={item.app.logo_url} size={size} />
    );
}

export function MakerLine({ item, className }: { item: StoreItem; className?: string }) {
    return (
        <span className={cn("inline-flex items-center gap-1 min-w-0 text-[11.5px] text-slate-400", className)}>
            <span className="truncate">{developerLabel(item)}</span>
            {item.kind === "builtin" && <BadgeCheckIcon className="w-3 h-3 text-sky-600 shrink-0" aria-label="Official" />}
        </span>
    );
}

// The one tag an app may carry; anything else is told by where it is listed.
export function ItemTag({ item }: { item: StoreItem }) {
    let label: string | null = null;
    let tone = "bg-slate-100 text-slate-500";
    if (item.kind === "community" && item.app.status === "featured") {
        label = "Featured";
        tone = "bg-sky-50 text-sky-700";
    } else if (item.kind === "builtin" && item.entry.beta) {
        label = "Beta";
    }
    if (!label) return null;
    return <span className={cn("h-[18px] px-1.5 rounded text-[10.5px] font-medium inline-flex items-center shrink-0", tone)}>{label}</span>;
}

export function ActionButton({
    item,
    connection,
    onAction,
}: {
    item: StoreItem;
    connection?: IntegrationConnection;
    onAction?: () => void;
}) {
    let label: React.ReactNode;
    let disabled = false;
    if (item.kind === "builtin") {
        if (connection) {
            label = (
                <>
                    <span className={cn("size-1.5 rounded-full", connection.status === "connected" ? "bg-emerald-500" : "bg-amber-500")} />
                    Manage
                </>
            );
        } else if (!isUsable(item.entry)) {
            label = "Coming soon";
            disabled = true;
        } else {
            label = "Connect";
        }
    } else {
        label = item.app.installed ? "Installed" : "View";
    }
    return (
        <button
            type="button"
            disabled={disabled}
            onClick={(e) => {
                e.stopPropagation();
                onAction?.();
            }}
            className={cn(
                "h-7 px-2.5 rounded-md border text-[12px] font-medium inline-flex items-center gap-1.5 shrink-0 transition-colors",
                disabled
                    ? "border-transparent text-slate-400 cursor-not-allowed"
                    : "border-slate-200 text-slate-700 hover:border-slate-300 hover:text-slate-900",
            )}
        >
            {label}
        </button>
    );
}

export interface AppCardProps {
    item: StoreItem;
    connection?: IntegrationConnection;
    onOpen: () => void;
    onAction: () => void;
    /** The search query, to mark what matched. */
    highlight?: string;
}

function openOnKey(onOpen: () => void) {
    return (e: React.KeyboardEvent) => {
        if (e.key === "Enter") onOpen();
    };
}

export default function AppCard({ item, connection, onOpen, onAction, highlight }: AppCardProps) {
    return (
        <div
            role="link"
            tabIndex={0}
            onClick={onOpen}
            onKeyDown={openOnKey(onOpen)}
            className="flex flex-col gap-3 rounded-lg border border-slate-200 bg-white p-4 cursor-pointer outline-none transition-colors hover:border-slate-300 focus-visible:border-sky-400"
        >
            <div className="flex items-center gap-3">
                <ItemLogo item={item} size={10} />
                <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5 min-w-0">
                        <span className="text-[13px] font-semibold text-slate-900 truncate">
                            <Highlight text={item.name} q={highlight} />
                        </span>
                        <ItemTag item={item} />
                    </div>
                    <MakerLine item={item} />
                </div>
            </div>
            <p className="text-[12.5px] text-slate-500 leading-relaxed line-clamp-2 min-h-[2.4rem]">
                <Highlight text={item.tagline} q={highlight} />
            </p>
            <div className="flex items-center justify-between gap-2">
                <span className="text-[11.5px] text-slate-400 truncate">{categoryLabel(item.category)}</span>
                <ActionButton item={item} connection={connection} onAction={onAction} />
            </div>
        </div>
    );
}

export function AppRow({ item, connection, onOpen, onAction, highlight }: AppCardProps) {
    return (
        <div
            role="link"
            tabIndex={0}
            onClick={onOpen}
            onKeyDown={openOnKey(onOpen)}
            className="flex items-center gap-3.5 px-4 py-3 cursor-pointer outline-none transition-colors hover:bg-slate-50 focus-visible:bg-slate-50"
        >
            <ItemLogo item={item} size={9} />
            <div className="min-w-0 flex-1">
                <div className="flex items-center gap-1.5 min-w-0">
                    <span className="text-[13px] font-medium text-slate-900 truncate">
                        <Highlight text={item.name} q={highlight} />
                    </span>
                    <ItemTag item={item} />
                </div>
                <p className="text-[12px] text-slate-500 truncate">
                    <Highlight text={item.tagline} q={highlight} />
                </p>
            </div>
            <MakerLine item={item} className="hidden lg:inline-flex w-36 shrink-0" />
            <span className="hidden md:block w-28 shrink-0 text-[12px] text-slate-400 truncate">{categoryLabel(item.category)}</span>
            <ActionButton item={item} connection={connection} onAction={onAction} />
        </div>
    );
}
