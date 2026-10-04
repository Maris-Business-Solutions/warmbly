// The connected CRMs a workspace can run on, and what each calls things. Every
// CRM surface reads this through useCrmProvider, so HubSpot and Pipedrive mode
// share one set of screens with each provider's own logo and words.

import { ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";
import type { CRMExternalRef } from "@/lib/api/models/app/crm/CRMProvider";
import { HubSpotMark } from "./HubSpot";

export type ExternalCrm = "hubspot" | "pipedrive";

export interface CrmInfo {
    id: ExternalCrm;
    name: string;
    settingsPath: string;
    // Tailwind classes in the provider's colour.
    tint: string;
    tintText: string;
    hoverText: string;
    hoverBg: string;
    border: string;
    words: {
        contact: string;
        contacts: string;
        company: string;
        owner: string;
        owners: string;
        task: string;
        tasks: string;
        list: string;
        lists: string;
        property: string;
        properties: string;
        // What a contact's lifecycle stage is called ("Label" in Pipedrive).
        lifecycle: string;
        lifecycles: string;
        // Pipedrive has no lead status.
        leadStatus: string | null;
        amount: string;
        closeDate: string;
    };
    // HubSpot closes a deal by moving it to a won or lost stage; Pipedrive
    // keeps the stage and marks the deal won or lost.
    wonLostAreStages: boolean;
}

export const CRM_INFO: Record<ExternalCrm, CrmInfo> = {
    hubspot: {
        id: "hubspot",
        name: "HubSpot",
        settingsPath: "/app/integrations/hubspot",
        tint: "bg-orange-50",
        tintText: "text-orange-700",
        hoverText: "hover:text-orange-700",
        hoverBg: "hover:bg-orange-50",
        border: "border-orange-200",
        words: {
            contact: "contact",
            contacts: "contacts",
            company: "company",
            owner: "owner",
            owners: "owners",
            task: "task",
            tasks: "tasks",
            list: "list",
            lists: "lists",
            property: "property",
            properties: "properties",
            lifecycle: "Lifecycle stage",
            lifecycles: "lifecycle stages",
            leadStatus: "Lead status",
            amount: "Amount",
            closeDate: "Close date",
        },
        wonLostAreStages: true,
    },
    pipedrive: {
        id: "pipedrive",
        name: "Pipedrive",
        settingsPath: "/app/integrations/pipedrive",
        tint: "bg-emerald-50",
        tintText: "text-emerald-700",
        hoverText: "hover:text-emerald-700",
        hoverBg: "hover:bg-emerald-50",
        border: "border-emerald-200",
        words: {
            contact: "person",
            contacts: "people",
            company: "organization",
            owner: "user",
            owners: "users",
            task: "activity",
            tasks: "activities",
            list: "filter",
            lists: "filters",
            property: "field",
            properties: "fields",
            lifecycle: "Label",
            lifecycles: "labels",
            leadStatus: null,
            amount: "Value",
            closeDate: "Expected close date",
        },
        wonLostAreStages: false,
    },
};

export function crmInfo(provider?: string | null): CrmInfo {
    return provider === "pipedrive" ? CRM_INFO.pipedrive : CRM_INFO.hubspot;
}

// Pipedrive's monogram: the white "P" on its green tile.
const PIPEDRIVE_P =
    "M59.6807,81.1772 C59.6807,101.5343 70.0078,123.4949 92.7336,123.4949 C109.5872,123.4949 126.6277,110.3374 126.6277,80.8785 C126.6277,55.0508 113.232,37.7119 93.2944,37.7119 C77.0483,37.7119 59.6807,49.1244 59.6807,81.1772 Z M101.3006,0 C142.0482,0 169.4469,32.2728 169.4469,80.3126 C169.4469,127.5978 140.584,160.60942 99.3224,160.60942 C79.6495,160.60942 67.0483,152.1836 60.4595,146.0843 C60.5063,147.5305 60.5374,149.1497 60.5374,150.8788 L60.5374,215 L18.32565,215 L18.32565,44.157 C18.32565,41.6732 17.53126,40.8873 15.07021,40.8873 L0.5531,40.8873 L0.5531,3.4741 L35.9736,3.4741 C52.282,3.4741 56.4564,11.7741 57.2508,18.1721 C63.8708,10.7524 77.5935,0 101.3006,0 Z";

export function PipedriveMark({ className, title = "Pipedrive" }: { className?: string; title?: string }) {
    return (
        <svg viewBox="0 0 304 304" role="img" aria-label={title} className={cn("w-3.5 h-3.5 shrink-0", className)}>
            <title>{title}</title>
            <rect width="304" height="304" rx="64" fill="#08A742" />
            <g transform="translate(67,44)">
                <path fill="#fff" d={PIPEDRIVE_P} />
            </g>
        </svg>
    );
}

// The connected CRM's logo.
export function CrmMark({ provider, className, title }: { provider?: string | null; className?: string; title?: string }) {
    const info = crmInfo(provider);
    if (info.id === "pipedrive") return <PipedriveMark className={className} title={title ?? info.name} />;
    return <HubSpotMark className={className} title={title ?? info.name} />;
}

// A small provider chip for section headers and pickers.
export function CrmBadge({ provider, className, label }: { provider?: string | null; className?: string; label?: string }) {
    const info = crmInfo(provider);
    return (
        <span
            className={cn(
                "inline-flex items-center gap-1 h-5 px-1.5 rounded-md text-[10.5px] font-medium shrink-0",
                info.tint,
                info.tintText,
                className,
            )}
        >
            <CrmMark provider={info.id} className="w-3 h-3" />
            {label ?? info.name}
        </span>
    );
}

// "Open in HubSpot / Pipedrive" for a record. Renders nothing without a link.
// Stops propagation so it never also opens the row it sits in.
export function OpenInCrm({
    url,
    external,
    provider,
    label,
    compact = false,
    className,
}: {
    url?: string;
    external?: CRMExternalRef;
    provider?: string | null;
    label?: string;
    compact?: boolean;
    className?: string;
}) {
    const info = crmInfo(provider ?? external?.provider);
    const href = url ?? external?.url;
    if (!href) return null;
    const text = label ?? `Open in ${info.name}`;
    return (
        <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            title={text}
            aria-label={text}
            className={cn(
                "inline-flex items-center gap-1 text-[12px] text-slate-600 transition-colors shrink-0",
                info.hoverText,
                info.hoverBg,
                compact ? "h-6 w-6 justify-center rounded-md" : "h-7 px-2 rounded-md",
                className,
            )}
        >
            <CrmMark provider={info.id} className="w-3.5 h-3.5" />
            {!compact && <span>{text}</span>}
            {!compact && <ExternalLink className="w-3 h-3 opacity-60" />}
        </a>
    );
}

// "Synced with Pipedrive · 2m ago", for footers and panel headers.
export function CrmSyncedAt({ at, provider, className }: { at?: Date | string; provider?: string | null; className?: string }) {
    if (!at) return null;
    const info = crmInfo(provider);
    const d = typeof at === "string" ? new Date(at) : at;
    return (
        <span className={cn("inline-flex items-center gap-1 text-[11px] text-slate-400", className)}>
            <CrmMark provider={info.id} className="w-3 h-3 opacity-80" />
            Synced with {info.name} · {timeAgo(d)}
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
