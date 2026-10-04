import { ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";
import type { CRMExternalRef } from "@/lib/api/models/app/crm/CRMProvider";

// HubSpot's brand: the sprocket in HubSpot orange. Used wherever a record or a
// section is HubSpot's, so it is obvious at a glance where data lives.
export const HUBSPOT_ORANGE = "#FF7A59";

const SPROCKET =
    "M18.164 7.93V5.084a2.198 2.198 0 001.267-1.978v-.067A2.2 2.2 0 0017.238.845h-.067a2.2 2.2 0 00-2.193 2.193v.067a2.196 2.196 0 001.252 1.973l.013.006v2.852a6.22 6.22 0 00-2.969 1.31l.012-.01-7.828-6.095A2.497 2.497 0 104.3 4.656l-.012.006 7.697 5.991a6.176 6.176 0 00-1.038 3.446c0 1.343.425 2.588 1.147 3.607l-.013-.02-2.342 2.343a1.968 1.968 0 00-.58-.095h-.002a2.033 2.033 0 102.033 2.033 1.978 1.978 0 00-.1-.595l.005.014 2.317-2.317a6.247 6.247 0 104.782-11.134l-.036-.005zm-.964 9.378a3.206 3.206 0 113.215-3.207v.002a3.206 3.206 0 01-3.207 3.207z";

export function HubSpotMark({ className, title = "HubSpot" }: { className?: string; title?: string }) {
    return (
        <svg viewBox="0 0 24 24" role="img" aria-label={title} className={cn("w-3.5 h-3.5 shrink-0", className)} fill={HUBSPOT_ORANGE}>
            <title>{title}</title>
            <path d={SPROCKET} />
        </svg>
    );
}

// A small "HubSpot" chip for section headers and pickers.
export function HubSpotBadge({ className, label = "HubSpot" }: { className?: string; label?: string }) {
    return (
        <span
            className={cn(
                "inline-flex items-center gap-1 h-5 px-1.5 rounded-md bg-orange-50 text-orange-700 text-[10.5px] font-medium shrink-0",
                className,
            )}
        >
            <HubSpotMark className="w-3 h-3" />
            {label}
        </span>
    );
}

// "Open in HubSpot" for a record. Renders nothing without a link. Stops
// propagation so it never also opens the row it sits in.
export function OpenInHubSpot({
    url,
    external,
    label = "Open in HubSpot",
    compact = false,
    className,
}: {
    url?: string;
    external?: CRMExternalRef;
    label?: string;
    compact?: boolean;
    className?: string;
}) {
    const href = url ?? external?.url;
    if (!href) return null;
    return (
        <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            title={label}
            aria-label={label}
            className={cn(
                "inline-flex items-center gap-1 text-[12px] text-slate-600 hover:text-orange-700 transition-colors shrink-0",
                compact ? "h-6 w-6 justify-center rounded-md hover:bg-orange-50" : "h-7 px-2 rounded-md hover:bg-orange-50",
                className,
            )}
        >
            <HubSpotMark className="w-3.5 h-3.5" />
            {!compact && <span>{label}</span>}
            {!compact && <ExternalLink className="w-3 h-3 opacity-60" />}
        </a>
    );
}

// "Synced with HubSpot · 2m ago", for footers and panel headers.
export function HubSpotSyncedAt({ at, className }: { at?: Date | string; className?: string }) {
    if (!at) return null;
    const d = typeof at === "string" ? new Date(at) : at;
    return (
        <span className={cn("inline-flex items-center gap-1 text-[11px] text-slate-400", className)}>
            <HubSpotMark className="w-3 h-3 opacity-80" />
            Synced with HubSpot · {timeAgo(d)}
        </span>
    );
}

function timeAgo(d: Date): string {
    const s = Math.max(0, Math.round((Date.now() - d.getTime()) / 1000));
    if (s < 45) return "just now";
    const m = Math.round(s / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.round(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.round(h / 24)}d ago`;
}
