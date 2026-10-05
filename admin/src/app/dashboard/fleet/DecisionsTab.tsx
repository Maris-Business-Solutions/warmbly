// Decision log: what the placement and rebalance loops did and why. Filtered
// server-side by kind and worker; before/after JSON expands inline. No event
// fires when a loop writes a decision, so the list polls at 30s.

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { ChevronDown, ChevronRight, ListChecks } from "lucide-react";
import { EmptyState, StatusBadge } from "@/components/ui/kit";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ErrorState";
import { SelectFilter } from "@/components/data/Explorer";
import { listFleetDecisions, type AdminFleetDecision } from "@/lib/api/client/admin/fleet";
import { listFleetNodes } from "@/lib/api/client/admin/fleetNodes";
import { cn } from "@/lib/utils";
import { fmtAgo, fmtDateTime, shortId } from "./format";

const LIMIT = 200;

function pretty(v: unknown): string {
    if (v == null) return "";
    try {
        return JSON.stringify(v, null, 2);
    } catch {
        return String(v);
    }
}

function DiffPane({ label, value }: { label: string; value: unknown }) {
    return (
        <div className="min-w-0">
            <div className="mb-1.5 text-xs font-medium text-muted-foreground">{label}</div>
            <pre className="max-h-64 overflow-auto rounded-md border border-border bg-card p-2.5 font-mono text-[11.5px] leading-relaxed text-foreground">
                {pretty(value) || "(none)"}
            </pre>
        </div>
    );
}

function DecisionRow({ d }: { d: AdminFleetDecision }) {
    const [open, setOpen] = useState(false);
    const hasDiff = d.before != null || d.after != null;
    return (
        <>
            <tr
                onClick={hasDiff ? () => setOpen((v) => !v) : undefined}
                className={cn(
                    "h-10 border-b border-border/70 transition-colors hover:bg-accent/50",
                    hasDiff && "cursor-pointer",
                    open && "bg-accent/40",
                )}
            >
                <td className="py-2 pr-3 pl-4 whitespace-nowrap text-muted-foreground tabular-nums" title={fmtDateTime(d.created_at)}>
                    {fmtAgo(d.created_at)}
                </td>
                <td className="px-3 py-2">
                    <StatusBadge className="font-mono">{d.kind}</StatusBadge>
                </td>
                <td className="px-3 py-2 whitespace-nowrap">
                    {d.worker_id ? (
                        <Link
                            to={`/workers/${d.worker_id}`}
                            onClick={(e) => e.stopPropagation()}
                            className="font-medium text-foreground underline-offset-2 hover:underline"
                        >
                            {d.worker_name || shortId(d.worker_id)}
                        </Link>
                    ) : (
                        <span className="text-subtle-foreground">—</span>
                    )}
                </td>
                <td className="px-3 py-2 font-mono text-xs text-muted-foreground">{shortId(d.mailbox_id)}</td>
                <td className="max-w-md px-3 py-2 text-foreground">
                    {d.reason || <span className="text-subtle-foreground">—</span>}
                </td>
                <td className="px-3 py-2 whitespace-nowrap text-muted-foreground">{d.triggered_by || "—"}</td>
                <td className="py-2 pr-4 pl-3 text-right">
                    {hasDiff && (
                        <button
                            type="button"
                            aria-expanded={open}
                            onClick={(e) => {
                                e.stopPropagation();
                                setOpen((v) => !v);
                            }}
                            className="inline-flex h-6 items-center gap-1 rounded-md px-1.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                        >
                            {open ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                            {open ? "Hide" : "Diff"}
                        </button>
                    )}
                </td>
            </tr>
            {open && hasDiff && (
                <tr className="border-b border-border/70 bg-muted/30">
                    <td colSpan={7} className="px-4 py-3">
                        <div className="grid gap-3 md:grid-cols-2">
                            <DiffPane label="Before" value={d.before} />
                            <DiffPane label="After" value={d.after} />
                        </div>
                    </td>
                </tr>
            )}
        </>
    );
}

export function DecisionsTab() {
    const [kind, setKind] = useState("");
    const [workerId, setWorkerId] = useState("");

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "fleet", "decisions", kind, workerId],
        queryFn: () => listFleetDecisions({ kind: kind || undefined, worker_id: workerId || undefined, limit: LIMIT }),
        refetchInterval: 30_000,
    });

    // The kind facet comes from the unfiltered log so an option never
    // disappears when it is selected; same key as the base query, so it is
    // one request when no filter is active.
    const kindsQ = useQuery({
        queryKey: ["admin", "fleet", "decisions", "", ""],
        queryFn: () => listFleetDecisions({ limit: LIMIT }),
        staleTime: 60_000,
    });
    const kinds = Array.from(new Set((kindsQ.data?.data ?? []).map((d) => d.kind))).sort();

    const workersQ = useQuery({
        queryKey: ["admin", "workers", "managed"],
        queryFn: () => listFleetNodes("worker"),
        staleTime: 60_000,
    });
    const workers = workersQ.data?.data ?? [];

    const rows = data?.data ?? [];

    return (
        <div>
            <div className="mb-4 flex flex-wrap items-end gap-3">
                <div className="w-full sm:w-52">
                    <div className="mb-1.5 text-xs font-medium text-muted-foreground">Kind</div>
                    <SelectFilter
                        value={kind || "any"}
                        onChange={(v) => setKind(v === "any" ? "" : v)}
                        options={[{ value: "any", label: "Any kind" }, ...kinds.map((k) => ({ value: k, label: k }))]}
                    />
                </div>
                <div className="w-full sm:w-64">
                    <div className="mb-1.5 text-xs font-medium text-muted-foreground">Worker</div>
                    <SelectFilter
                        value={workerId || "any"}
                        onChange={(v) => setWorkerId(v === "any" ? "" : v)}
                        options={[
                            { value: "any", label: "Any worker" },
                            ...workers.map((w) => ({ value: w.id, label: w.name || w.id.slice(0, 8) })),
                        ]}
                    />
                </div>
                <div className="pb-1.5 text-xs text-muted-foreground tabular-nums sm:ml-auto">
                    {isLoading ? "Loading…" : `${rows.length} decision${rows.length === 1 ? "" : "s"}`}
                    {rows.length >= LIMIT && " (newest " + LIMIT + ")"}
                </div>
            </div>

            {error ? (
                <ErrorState error={error} title="Failed to load the decision log" onRetry={() => refetch()} />
            ) : isLoading ? (
                <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
                    <div className="h-9 border-b border-border" />
                    {Array.from({ length: 6 }).map((_, i) => (
                        <div key={i} className="flex h-10 items-center border-b border-border/70 px-4 last:border-b-0">
                            <Skeleton className="h-3.5 w-full" />
                        </div>
                    ))}
                </div>
            ) : rows.length === 0 ? (
                <div className="surface-lit rounded-xl border border-border bg-card">
                    <EmptyState
                        icon={ListChecks}
                        title={`No decisions recorded${kind || workerId ? " for these filters" : ""}`}
                        hint="Rows appear when the assignment loop places a mailbox, the rebalancer moves one, or a worker's health changes."
                    />
                </div>
            ) : (
                <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
                    <div className="overflow-x-auto">
                        <table className="w-full text-[13px]">
                            <thead>
                                <tr className="h-9 border-b border-border text-left text-xs font-medium text-muted-foreground">
                                    <th className="pr-3 pl-4 font-medium">When</th>
                                    <th className="px-3 font-medium">Kind</th>
                                    <th className="px-3 font-medium">Worker</th>
                                    <th className="px-3 font-medium">Mailbox</th>
                                    <th className="px-3 font-medium">Reason</th>
                                    <th className="px-3 font-medium whitespace-nowrap">Triggered by</th>
                                    <th className="pr-4 pl-3" />
                                </tr>
                            </thead>
                            <tbody className="[&>tr:last-child]:border-b-0">
                                {rows.map((d) => (
                                    <DecisionRow key={d.id} d={d} />
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}
        </div>
    );
}
