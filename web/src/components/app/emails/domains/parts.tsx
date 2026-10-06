// Pieces the Sending domains page, its drawer and the import wizards share.
import React from "react";
import { CheckIcon, CircleDashedIcon, CopyIcon } from "lucide-react";
import toast from "react-hot-toast/headless";
import type { SendingDomain, TrackingSuggestion } from "@/lib/api/models/app/emails/SendingDomain";
import { vendorLabel } from "@/lib/api/models/app/emails/MailboxSources";
import ProviderLogo from "@/components/app/emails/ProviderLogo";
import { cn } from "@/lib/utils";
import { domainVendor, type PillTone } from "./rules";

export function AuthDot({ on, label, state, hint }: { on: boolean; label: string; state?: string; hint?: string }) {
    // An unchecked domain is pale whatever the flags say, so "not found" is never claimed before a check.
    const known = state !== "unknown";
    return (
        <span
            className="inline-flex items-center gap-1 text-[10.5px] text-slate-500"
            title={!known ? `${label} not checked yet` : on ? `${label} found` : hint ?? `${label} not found`}
        >
            <span className={cn("size-1.5 rounded-full", !known ? "bg-slate-200" : on ? "bg-emerald-500" : "bg-slate-300")} />
            {label}
        </span>
    );
}

const DKIM_HINT = "A DKIM key sits at a selector DNS cannot list, so not found is unconfirmed rather than missing.";

export function AuthDots({
    spf,
    dkim,
    dmarc,
    state,
    className,
}: {
    spf: boolean;
    dkim: boolean;
    dmarc: boolean;
    state?: string;
    className?: string;
}) {
    return (
        <span className={cn("inline-flex items-center gap-2.5", className)}>
            <AuthDot on={spf} label="SPF" state={state} />
            <AuthDot on={dkim} label="DKIM" state={state} hint={DKIM_HINT} />
            <AuthDot on={dmarc} label="DMARC" state={state} />
        </span>
    );
}

const AUTH_STATE: Record<string, { cls: string; label: string }> = {
    passing: { cls: "text-emerald-700", label: "Passing" },
    failing: { cls: "text-rose-700", label: "Failing" },
    unknown: { cls: "text-slate-400", label: "Not checked" },
};

export function AuthStateLabel({ state }: { state: string }) {
    const s = AUTH_STATE[state] ?? AUTH_STATE.unknown;
    return <span className={cn("text-[11px] font-medium", s.cls)}>{s.label}</span>;
}

export function Chip({ tone, children, title }: { tone: "emerald" | "amber" | "slate" | "sky"; children: React.ReactNode; title?: string }) {
    const cls = {
        emerald: "text-emerald-700 bg-emerald-50",
        amber: "text-amber-700 bg-amber-50",
        slate: "text-slate-600 bg-slate-100",
        sky: "text-sky-700 bg-sky-50",
    }[tone];
    return (
        <span title={title} className={cn("inline-flex items-center h-[18px] px-1.5 rounded text-[10.5px] font-medium whitespace-nowrap", cls)}>
            {children}
        </span>
    );
}

const SUGGESTION_CHIP: Record<TrackingSuggestion["status"], { tone: "emerald" | "sky" | "slate"; label: string; title: string }> = {
    active: { tone: "emerald", label: "Active", title: "A mailbox on this domain already uses it, verified." },
    found: { tone: "emerald", label: "Ready", title: "Its DNS already points at Warmbly, ready to use." },
    suggested: { tone: "slate", label: "Needs DNS", title: "Add its CNAME record; it switches on by itself once it resolves." },
};

export function SuggestionChip({ status }: { status: TrackingSuggestion["status"] }) {
    const c = SUGGESTION_CHIP[status] ?? SUGGESTION_CHIP.suggested;
    return (
        <Chip tone={c.tone} title={c.title}>
            {c.label}
        </Chip>
    );
}

export function CopyButton({ value, label }: { value: string; label: string }) {
    const [copied, setCopied] = React.useState(false);
    React.useEffect(() => {
        if (!copied) return;
        const t = window.setTimeout(() => setCopied(false), 1500);
        return () => window.clearTimeout(t);
    }, [copied]);
    const copy = async (e: React.MouseEvent) => {
        e.stopPropagation();
        try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
        } catch {
            toast.error("Could not copy. Select the text and copy it by hand.");
        }
    };
    return (
        <button
            type="button"
            onClick={(e) => void copy(e)}
            aria-label={`Copy ${label}`}
            title={copied ? "Copied" : `Copy ${label}`}
            className={cn(
                "size-5 rounded inline-flex items-center justify-center shrink-0 transition-colors",
                copied ? "text-emerald-600 bg-emerald-50" : "text-slate-400 hover:text-slate-700 hover:bg-slate-100",
            )}
        >
            {copied ? <CheckIcon className="w-3 h-3" /> : <CopyIcon className="w-3 h-3" />}
        </button>
    );
}

export interface RecordRow {
    key: string;
    /** "Ownership", "Root", ...; omitted for a single record. */
    purpose?: string;
    type: string;
    name: string;
    value: string;
    /** undefined when there is nothing to check against yet. */
    ok?: boolean;
    optional?: boolean;
}

/** DNS records to add at the provider, each value with a copy button. */
export function DnsRecordsTable({ records, className }: { records: RecordRow[]; className?: string }) {
    const showPurpose = records.some((r) => r.purpose);
    const showState = records.some((r) => r.ok !== undefined);
    return (
        <div className={cn("rounded-md border border-slate-200 overflow-hidden", className)}>
            <div className="divide-y divide-slate-100">
                {records.map((r) => (
                    <div key={r.key} className="px-2.5 py-2 space-y-1">
                        <div className="flex items-center gap-1.5 min-w-0">
                            {showPurpose && r.purpose && <span className="text-[11.5px] font-medium text-slate-800">{r.purpose}</span>}
                            <Chip tone="slate">{r.type}</Chip>
                            {r.optional && <Chip tone="sky">Optional</Chip>}
                            {showState && r.ok !== undefined && (
                                <span className="ml-auto shrink-0">
                                    {r.ok ? (
                                        <span className="inline-flex items-center gap-1 text-[11px] text-emerald-700">
                                            <CheckIcon className="w-3 h-3" />
                                            In place
                                        </span>
                                    ) : (
                                        <span className="inline-flex items-center gap-1 text-[11px] text-slate-500">
                                            <CircleDashedIcon className="w-3 h-3" />
                                            Not seen yet
                                        </span>
                                    )}
                                </span>
                            )}
                        </div>
                        <RecordField label="Name" value={r.name} />
                        <RecordField label="Value" value={r.value} />
                    </div>
                ))}
            </div>
        </div>
    );
}

function RecordField({ label, value }: { label: string; value: string }) {
    return (
        <div className="flex items-center gap-2 min-w-0">
            <span className="w-10 shrink-0 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{label}</span>
            <span className="min-w-0 flex-1 text-[11.5px] font-mono text-slate-800 break-all select-all">{value}</span>
            <CopyButton value={value} label={label.toLowerCase()} />
        </div>
    );
}


const PILL_TONE: Record<PillTone, { pill: string; dot: string }> = {
    emerald: { pill: "bg-emerald-50 text-emerald-700 ring-emerald-200/70", dot: "bg-emerald-500" },
    amber: { pill: "bg-amber-50 text-amber-700 ring-amber-200/70", dot: "bg-amber-500" },
    rose: { pill: "bg-rose-50 text-rose-700 ring-rose-200/70", dot: "bg-rose-500" },
    slate: { pill: "bg-slate-50 text-slate-500 ring-slate-200", dot: "bg-slate-300" },
    sky: { pill: "bg-sky-50 text-sky-700 ring-sky-200/70", dot: "bg-sky-500" },
};

/** One status as a rounded pill with a dot; the same tones on the list and in the drawer. */
export function StatusPill({
    tone,
    children,
    title,
    onClick,
    className,
}: {
    tone: PillTone;
    children: React.ReactNode;
    title?: string;
    onClick?: () => void;
    className?: string;
}) {
    const t = PILL_TONE[tone];
    const cls = cn(
        "inline-flex items-center gap-1.5 h-5 px-2 rounded-full ring-1 ring-inset text-[10.5px] font-medium whitespace-nowrap min-w-0 max-w-full",
        t.pill,
        onClick && "hover:brightness-[0.97] transition-[filter]",
        className,
    );
    const inner = (
        <>
            <span className={cn("size-1.5 rounded-full shrink-0", t.dot)} />
            <span className="truncate">{children}</span>
        </>
    );
    if (!onClick) {
        return (
            <span title={title} className={cls}>
                {inner}
            </span>
        );
    }
    return (
        <button
            type="button"
            title={title}
            onClick={(e) => {
                e.stopPropagation();
                onClick();
            }}
            className={cls}
        >
            {inner}
        </button>
    );
}

/** "Managed by Zapmail" when a connected vendor account holds the domain, else "From Zapmail". */
export function VendorChip({ d, compact = false, className }: { d: SendingDomain; compact?: boolean; className?: string }) {
    const vendor = domainVendor(d);
    if (!vendor) return null;
    const name = vendorLabel(vendor);
    const managed = !!d.vendor_domain;
    return (
        <span
            title={managed ? `${name} holds this domain` : `Mailboxes imported from ${name}`}
            className={cn(
                "inline-flex items-center gap-1 h-[18px] pl-0.5 pr-1.5 rounded-full border text-[10.5px] font-medium whitespace-nowrap shrink-0",
                managed ? "border-sky-200 bg-sky-50 text-sky-800" : "border-slate-200 bg-slate-50 text-slate-600",
                className,
            )}
        >
            <ProviderLogo id={vendor} size="xs" framed={false} title="" />
            {compact ? name : managed ? `Managed by ${name}` : `From ${name}`}
        </span>
    );
}
