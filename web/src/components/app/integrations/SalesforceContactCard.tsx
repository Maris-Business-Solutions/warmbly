// The Salesforce side of a contact: the linked Lead or Contact, its owner,
// status, open opportunities and recent tasks, read live from Salesforce.
// Shown in the contact drawer and, compact, in the unibox contact rail.
// Renders nothing when the workspace has no Salesforce connection.

import React from "react";
import { Link } from "@tanstack/react-router";
import {
    AlertTriangleIcon,
    ExternalLinkIcon,
    Loader2Icon,
    PlusIcon,
    RefreshCwIcon,
    Settings2Icon,
    UnlinkIcon,
} from "lucide-react";
import toast from "react-hot-toast/headless";

import { Logo } from "@/components/svg";
import { SelectMenu } from "@/components/ui/select-menu";
import { useConfirm } from "@/hooks/context/confirm";
import {
    useContactSalesforce,
    useSyncContactSalesforce,
    useUnlinkContactSalesforce,
} from "@/lib/api/hooks/app/integrations/useSalesforce";
import type {
    ContactSalesforcePanel,
    ContactSalesforceRecord,
} from "@/lib/api/models/app/integrations/Salesforce";
import { errorMessage } from "@/lib/errors/message";
import { cn } from "@/lib/utils";

type Variant = "full" | "compact";

const NO_PERMISSION = "You need the integrations permission to change Salesforce records";

export default function SalesforceContactCard({
    contactId,
    variant = "full",
}: {
    contactId: string;
    variant?: Variant;
}) {
    const q = useContactSalesforce(contactId);
    const status = (q.error as { status?: number } | null)?.status;

    // No connection, no access, or nothing to show: stay out of the way.
    if (q.isPending) return null;
    if (q.isError && (status === 403 || status === 404)) return null;
    if (q.isError) {
        return (
            <Frame variant={variant}>
                <div className="flex items-center gap-2 text-[11.5px] text-slate-500">
                    <AlertTriangleIcon className="w-3 h-3 text-amber-500 shrink-0" />
                    <span className="flex-1 min-w-0 truncate">{errorMessage(q.error, "Could not reach Salesforce")}</span>
                    <button
                        type="button"
                        onClick={() => void q.refetch()}
                        className="text-[11px] text-sky-700 hover:underline shrink-0"
                    >
                        Retry
                    </button>
                </div>
            </Frame>
        );
    }
    const panel = q.data;
    if (!panel || panel.connections.length === 0) return null;

    return (
        <Frame variant={variant} refreshing={q.isFetching}>
            {panel.records.length === 0 ? (
                <NotInSalesforce contactId={contactId} panel={panel} variant={variant} />
            ) : (
                <div className={variant === "full" ? "space-y-2" : "space-y-3"}>
                    {panel.records.map((r) => (
                        <RecordCard
                            key={r.link_id}
                            contactId={contactId}
                            record={r}
                            canSync={panel.can_sync}
                            showConnection={panel.connections.length > 1}
                            variant={variant}
                        />
                    ))}
                </div>
            )}
        </Frame>
    );
}

function Frame({
    variant,
    refreshing,
    children,
}: {
    variant: Variant;
    refreshing?: boolean;
    children: React.ReactNode;
}) {
    const spinner = refreshing && <Loader2Icon className="w-3 h-3 animate-spin text-slate-300" />;
    if (variant === "compact") {
        return (
            <div className="px-4 py-3">
                <div className="flex items-center gap-2 mb-2">
                    <span className="text-[10.5px] uppercase tracking-[0.12em] text-slate-400 font-medium">Salesforce</span>
                    {spinner}
                </div>
                {children}
            </div>
        );
    }
    return (
        <section>
            <h2 className="text-[10px] uppercase tracking-[0.14em] font-semibold text-slate-500 mb-2 flex items-center gap-2">
                Salesforce
                {spinner}
            </h2>
            {children}
        </section>
    );
}

function NotInSalesforce({
    contactId,
    panel,
    variant,
}: {
    contactId: string;
    panel: ContactSalesforcePanel;
    variant: Variant;
}) {
    const sync = useSyncContactSalesforce(contactId);
    const [connectionId, setConnectionId] = React.useState(panel.connections[0]?.id ?? "");
    const [pending, setPending] = React.useState<"lead" | "contact" | null>(null);

    async function create(as: "lead" | "contact") {
        setPending(as);
        try {
            const res = await sync.mutateAsync({ connection_id: connectionId || undefined, create_as: as });
            toast.success(res.records.length > 0 ? `Added to Salesforce as a ${as === "lead" ? "Lead" : "Contact"}` : "Synced");
        } catch (err) {
            toast.error(errorMessage(err, "Could not add to Salesforce"));
        } finally {
            setPending(null);
        }
    }

    const btn =
        "h-7 px-2 rounded-md border border-slate-200 bg-white hover:border-slate-300 text-[11.5px] text-slate-700 hover:text-slate-900 inline-flex items-center gap-1.5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed";

    return (
        <div className={cn("rounded-md border border-dashed border-slate-300 bg-white", variant === "full" ? "px-3 py-3" : "px-2.5 py-2.5")}>
            <p className="text-[12px] font-medium text-slate-700">Not in Salesforce yet</p>
            <p className="text-[11px] text-slate-500 mt-0.5">
                No Lead or Contact with this email. Activity is logged once the record exists.
            </p>
            {panel.connections.length > 1 && (
                <div className="mt-2">
                    <SelectMenu
                        value={connectionId}
                        onChange={setConnectionId}
                        fullWidth
                        aria-label="Salesforce connection"
                        options={panel.connections.map((c) => ({
                            value: c.id,
                            label: c.environment === "sandbox" ? `${c.label} (sandbox)` : c.label,
                        }))}
                    />
                </div>
            )}
            <div className="mt-2 flex flex-wrap gap-1.5">
                {(["lead", "contact"] as const).map((as) => (
                    <button
                        key={as}
                        type="button"
                        onClick={() => void create(as)}
                        disabled={!panel.can_sync || pending !== null}
                        title={panel.can_sync ? undefined : NO_PERMISSION}
                        className={btn}
                    >
                        {pending === as ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <PlusIcon className="w-3 h-3" />}
                        {variant === "full" ? `Add to Salesforce as ${as === "lead" ? "Lead" : "Contact"}` : `As ${as === "lead" ? "Lead" : "Contact"}`}
                    </button>
                ))}
            </div>
        </div>
    );
}

function RecordCard({
    contactId,
    record: r,
    canSync,
    showConnection,
    variant,
}: {
    contactId: string;
    record: ContactSalesforceRecord;
    canSync: boolean;
    showConnection: boolean;
    variant: Variant;
}) {
    const confirm = useConfirm();
    const sync = useSyncContactSalesforce(contactId);
    const unlink = useUnlinkContactSalesforce(contactId);
    const compact = variant === "compact";
    const openOpps = r.opportunities.filter((o) => !o.is_closed);
    const opps = compact ? openOpps.slice(0, 2) : r.opportunities.slice(0, 5);
    const tasks = r.tasks.slice(0, compact ? 2 : 5);

    function syncNow() {
        sync.mutate(
            { connection_id: r.connection_id },
            {
                onSuccess: () => toast.success("Synced with Salesforce"),
                onError: (err) => toast.error(errorMessage(err, "Sync failed")),
            },
        );
    }
    function askUnlink() {
        confirm.show(
            `Unlink ${r.name || "this record"} from this contact? Nothing is deleted in Salesforce. Warmbly stops logging activity to it until it is matched again.`,
            async () => {
                try {
                    await unlink.mutateAsync(r.link_id);
                    toast.success("Unlinked");
                } catch (err) {
                    toast.error(errorMessage(err, "Could not unlink"));
                    throw err;
                }
            },
        );
    }

    const actionBtn =
        "h-6 px-1.5 rounded-md text-[11px] inline-flex items-center gap-1 transition-colors disabled:opacity-50 disabled:cursor-not-allowed";

    return (
        <div className="rounded-md border border-slate-200 bg-white overflow-hidden">
            <div className={cn("space-y-1.5", compact ? "px-2.5 py-2" : "px-3 py-2.5")}>
                <div className="flex items-start gap-2">
                    <span
                        className={cn(
                            "mt-px inline-flex items-center h-4 px-1 rounded text-[9.5px] uppercase tracking-[0.08em] font-semibold shrink-0",
                            r.object === "Lead" ? "bg-amber-50 text-amber-700" : "bg-sky-50 text-sky-700",
                        )}
                    >
                        {r.object}
                    </span>
                    <div className="min-w-0 flex-1">
                        <div className="text-[12.5px] font-medium text-slate-900 truncate">{r.name || r.email || "Unnamed"}</div>
                        {r.title && <div className="text-[11px] text-slate-500 truncate">{r.title}</div>}
                    </div>
                    <a
                        href={r.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        title="Open in Salesforce"
                        className="shrink-0 inline-flex items-center gap-1 text-[11px] text-sky-700 hover:underline"
                    >
                        {!compact && "Open in Salesforce"}
                        <ExternalLinkIcon className="w-3 h-3" />
                    </a>
                </div>

                {(r.account || r.company) && (
                    <div className="text-[11.5px] text-slate-600 truncate">
                        {r.account ? (
                            <a href={r.account.url} target="_blank" rel="noopener noreferrer" className="hover:text-sky-700 hover:underline">
                                {r.account.name}
                            </a>
                        ) : (
                            r.company
                        )}
                    </div>
                )}

                <div className="flex flex-wrap items-center gap-1">
                    {r.status && <Tag tone="slate">{r.status}</Tag>}
                    {r.is_converted && <Tag tone="emerald">Converted</Tag>}
                    {r.opted_out && <Tag tone="rose">Opted out</Tag>}
                    {r.stale && (
                        <Tag tone="amber" title={r.error || "Live refresh failed, showing the last known values"}>
                            Stale
                        </Tag>
                    )}
                    {showConnection && <Tag tone="slate">{r.connection_label}</Tag>}
                    {r.owner && <span className="text-[11px] text-slate-500 ml-0.5 truncate">Owner {r.owner.name}</span>}
                </div>

                {opps.length > 0 && (
                    <div className="pt-1 space-y-1">
                        <Sub>{compact ? "Open opportunities" : "Opportunities"}</Sub>
                        {opps.map((o) => (
                            <a
                                key={o.id}
                                href={o.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="flex items-center gap-2 text-[11.5px] rounded px-1 -mx-1 py-0.5 hover:bg-slate-50"
                            >
                                <span
                                    className={cn(
                                        "size-1.5 rounded-full shrink-0",
                                        o.is_won ? "bg-emerald-500" : o.is_closed ? "bg-slate-300" : "bg-sky-500",
                                    )}
                                />
                                <span className="text-slate-800 truncate flex-1 min-w-0">{o.name}</span>
                                <span className="text-slate-500 truncate max-w-[40%]">{o.stage}</span>
                                {!compact && o.amount != null && (
                                    <span className="text-slate-700 tabular-nums shrink-0">{money(o.amount)}</span>
                                )}
                                {!compact && o.close_date && (
                                    <span className="text-slate-400 tabular-nums shrink-0">{shortDate(o.close_date)}</span>
                                )}
                            </a>
                        ))}
                    </div>
                )}

                {tasks.length > 0 && (
                    <div className="pt-1 space-y-1">
                        <Sub>Recent activity in Salesforce</Sub>
                        {tasks.map((t) => (
                            <a
                                key={t.id}
                                href={t.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="flex items-center gap-2 text-[11.5px] rounded px-1 -mx-1 py-0.5 hover:bg-slate-50"
                            >
                                {t.from_warmbly ? (
                                    <span title="Logged by Warmbly" className="shrink-0 inline-flex">
                                        <Logo className="w-3 text-sky-600" />
                                    </span>
                                ) : (
                                    <span className="size-1.5 rounded-full bg-slate-300 shrink-0 mx-[3px]" />
                                )}
                                <span className="text-slate-700 truncate flex-1 min-w-0">{t.subject || "Task"}</span>
                                {!compact && t.owner_name && <span className="text-slate-400 truncate max-w-[30%]">{t.owner_name}</span>}
                                {t.date && <span className="text-slate-400 tabular-nums shrink-0">{shortDate(t.date)}</span>}
                            </a>
                        ))}
                    </div>
                )}
            </div>

            <div
                className={cn(
                    "border-t border-slate-100 bg-slate-50/60 flex flex-wrap items-center gap-x-2 gap-y-1",
                    compact ? "px-2.5 py-1.5" : "px-3 py-1.5",
                )}
            >
                <span className="text-[10.5px] text-slate-500 flex-1 min-w-0 truncate">
                    {r.last_synced_at ? `Synced ${ago(r.last_synced_at)}` : "Not synced yet"}
                    {r.sync.pending > 0 && ` · ${r.sync.pending} pending`}
                    {r.sync.failed > 0 && <span className="text-rose-600"> · {r.sync.failed} failed</span>}
                </span>
                <button
                    type="button"
                    onClick={syncNow}
                    disabled={!canSync || sync.isPending}
                    title={canSync ? "Push and pull this record now" : NO_PERMISSION}
                    className={cn(actionBtn, "text-slate-600 hover:text-slate-900 hover:bg-slate-100")}
                >
                    <RefreshCwIcon className={cn("w-3 h-3", sync.isPending && "animate-spin")} />
                    Sync now
                </button>
                <button
                    type="button"
                    onClick={askUnlink}
                    disabled={!canSync || unlink.isPending}
                    title={canSync ? undefined : NO_PERMISSION}
                    className={cn(actionBtn, "text-slate-500 hover:text-rose-600 hover:bg-rose-50")}
                >
                    <UnlinkIcon className="w-3 h-3" />
                    Unlink
                </button>
                {!compact && (
                    <Link
                        to="/app/integrations/salesforce/$id"
                        params={{ id: r.connection_id }}
                        title="Salesforce settings"
                        className={cn(actionBtn, "text-slate-400 hover:text-slate-900 hover:bg-slate-100")}
                    >
                        <Settings2Icon className="w-3 h-3" />
                    </Link>
                )}
            </div>
            {(r.sync.last_error || (r.stale && r.error)) && (
                <div className={cn("border-t border-rose-100 bg-rose-50/60 text-[10.5px] text-rose-700 break-words flex items-start gap-1.5", compact ? "px-2.5 py-1.5" : "px-3 py-1.5")}>
                    <AlertTriangleIcon className="w-3 h-3 mt-px shrink-0" />
                    <span>{r.sync.last_error || r.error}</span>
                </div>
            )}
        </div>
    );
}

function Sub({ children }: { children: React.ReactNode }) {
    return <div className="text-[9.5px] uppercase tracking-[0.12em] text-slate-400 font-medium">{children}</div>;
}

function Tag({
    tone,
    title,
    children,
}: {
    tone: "slate" | "emerald" | "rose" | "amber";
    title?: string;
    children: React.ReactNode;
}) {
    const cls = {
        slate: "bg-slate-100 text-slate-600 border-slate-200",
        emerald: "bg-emerald-50 text-emerald-700 border-emerald-100",
        rose: "bg-rose-50 text-rose-700 border-rose-100",
        amber: "bg-amber-50 text-amber-700 border-amber-200",
    }[tone];
    return (
        <span title={title} className={cn("inline-flex items-center h-[18px] px-1.5 rounded border text-[10px] font-medium max-w-full truncate", cls)}>
            {children}
        </span>
    );
}

function ago(d: string | Date): string {
    const t = new Date(d).getTime();
    if (Number.isNaN(t)) return "a while ago";
    const sec = Math.max(0, Math.round((Date.now() - t) / 1000));
    if (sec < 45) return "just now";
    const min = Math.round(sec / 60);
    if (min < 60) return `${min} min ago`;
    const hr = Math.round(min / 60);
    if (hr < 24) return `${hr} h ago`;
    return `${Math.round(hr / 24)} d ago`;
}

function shortDate(d: string): string {
    // Salesforce dates are YYYY-MM-DD; parse as local so they do not shift a day.
    const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(d);
    const dt = m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : new Date(d);
    if (Number.isNaN(dt.getTime())) return d;
    return dt.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

// The panel carries no currency code, so amounts show as plain numbers.
function money(n: number): string {
    return n.toLocaleString(undefined, { maximumFractionDigits: 0 });
}
