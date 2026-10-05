// Fleet capacity: every worker against its capacity-view row. There is no
// realtime event for the rolling 1h counters, so this view polls at 30s.

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { StatusBadge, StatusDot } from "@/components/ui/kit";
import { DataTable, type Column } from "@/components/data/DataTable";
import { StateLegend } from "@/components/StateLegend";
import { WORKER_HEALTH_LEGEND } from "@/lib/legends";
import { getFleetCapacity, type AdminFleetWorkerRow } from "@/lib/api/client/admin/fleet";
import { TONE_DOT, TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { HealthPill, LiveDot } from "./tones";
import { fmtAgo } from "./format";

function UtilizationBar({ row }: { row: AdminFleetWorkerRow }) {
    const u = row.utilization ?? 0;
    const pct = Math.max(0, Math.min(100, Math.round(u * 100)));
    const hot = u > 0.8;
    const cold = u < 0.5;
    return (
        <div className="min-w-[160px]">
            <div className="flex items-center justify-between text-xs tabular-nums">
                <span className="text-foreground">
                    {row.load_score.toFixed(0)}
                    <span className="text-subtle-foreground"> / {row.effective_capacity.toFixed(0)}</span>
                </span>
                <span
                    className={cn(
                        "font-medium",
                        hot ? TONE_TEXT.danger : cold ? "text-muted-foreground" : TONE_TEXT.success,
                    )}
                >
                    {pct}%
                </span>
            </div>
            <div className="mt-1 h-1 overflow-hidden rounded-full bg-border">
                <div
                    className={cn(
                        "h-full rounded-full",
                        hot ? TONE_DOT.danger : cold ? "bg-subtle-foreground/50" : TONE_DOT.success,
                    )}
                    style={{ width: `${pct}%` }}
                />
            </div>
        </div>
    );
}

function Pair({ a, b, tone, title }: { a: number; b: number; tone?: string; title?: string }) {
    return (
        <span className="tabular-nums" title={title}>
            <span className={tone}>{a.toLocaleString()}</span>
            <span className="text-subtle-foreground"> / {b.toLocaleString()}</span>
        </span>
    );
}

function Count({ n, warnAbove = 0 }: { n: number; warnAbove?: number }) {
    return (
        <span className={cn("tabular-nums", n > warnAbove ? cn("font-medium", TONE_TEXT.danger) : "text-subtle-foreground")}>
            {n.toLocaleString()}
        </span>
    );
}

const columns: Column<AdminFleetWorkerRow>[] = [
    {
        id: "name",
        header: "Worker",
        sortable: true,
        cell: (w) => (
            <div className="min-w-0">
                <Link
                    to={`/workers/${w.worker_id}`}
                    onClick={(e) => e.stopPropagation()}
                    className="font-medium text-foreground underline-offset-2 hover:underline"
                >
                    {w.name || w.worker_id.slice(0, 8)}
                </Link>
                <div className="font-mono text-[11px] text-subtle-foreground">{w.ip_addr}</div>
            </div>
        ),
        csv: (w) => w.name || w.worker_id,
    },
    {
        id: "region",
        header: "Region",
        cell: (w) =>
            w.region ? (
                <span className="font-mono text-xs text-muted-foreground">{w.region}</span>
            ) : (
                <span className="text-subtle-foreground">—</span>
            ),
        csv: (w) => w.region,
    },
    { id: "health", header: "Health", cell: (w) => <HealthPill state={w.health_state} />, csv: (w) => w.health_state },
    {
        id: "live",
        header: "Live",
        cell: (w) => <LiveDot live={w.live} title={w.last_seen_at ? `last seen ${fmtAgo(w.last_seen_at)}` : "no heartbeat"} />,
        csv: (w) => (w.live ? "live" : "offline"),
    },
    {
        id: "accounts",
        header: "Accounts",
        align: "right",
        sortable: true,
        cell: (w) => <span className="tabular-nums">{w.account_count}</span>,
        csv: (w) => w.account_count,
    },
    {
        id: "utilization",
        header: "Load / target",
        sortable: true,
        cell: (w) => <UtilizationBar row={w} />,
        csv: (w) => `${w.load_score.toFixed(0)}/${w.effective_capacity.toFixed(0)} (${Math.round(w.utilization * 100)}%)`,
    },
    {
        id: "sends",
        header: "Send activity 60m",
        align: "right",
        sortable: true,
        cell: (w) =>
            w.sends_attempted_1h === 0 ? (
                <span
                    className="text-subtle-foreground"
                    title="The worker made no provider send attempts during the rolling last 60 minutes"
                >
                    No attempts
                </span>
            ) : (
                <Pair
                    a={w.sends_succeeded_1h}
                    b={w.sends_attempted_1h}
                    tone="text-foreground"
                    title="Succeeded / attempted during the rolling last 60 minutes"
                />
            ),
        csv: (w) => `${w.sends_succeeded_1h}/${w.sends_attempted_1h}`,
    },
    {
        id: "bounces",
        header: "Bounces 1h",
        align: "right",
        cell: (w) => <Pair a={w.bounces_hard_1h} b={w.bounces_soft_1h} tone={w.bounces_hard_1h > 0 ? cn("font-medium", TONE_TEXT.danger) : "text-foreground"} />,
        csv: (w) => `${w.bounces_hard_1h} hard / ${w.bounces_soft_1h} soft`,
    },
    { id: "complaints", header: "Complaints 1h", align: "right", cell: (w) => <Count n={w.complaints_1h} />, csv: (w) => w.complaints_1h },
    { id: "auth", header: "Auth errors 1h", align: "right", cell: (w) => <Count n={w.auth_errors_1h} />, csv: (w) => w.auth_errors_1h },
    {
        id: "tags",
        header: "Tags",
        cell: (w) =>
            w.tags && w.tags.length ? (
                <div className="flex flex-wrap gap-1">
                    {w.tags.map((t) => (
                        <StatusBadge key={t}>{t}</StatusBadge>
                    ))}
                </div>
            ) : (
                <span className="text-subtle-foreground">—</span>
            ),
        csv: (w) => (w.tags || []).join(" "),
        defaultHidden: true,
    },
];

function compare(a: AdminFleetWorkerRow, b: AdminFleetWorkerRow, by: string): number {
    switch (by) {
        case "name":
            return (a.name || a.worker_id).localeCompare(b.name || b.worker_id);
        case "accounts":
            return a.account_count - b.account_count;
        case "utilization":
            return a.utilization - b.utilization;
        case "sends":
            return a.sends_attempted_1h - b.sends_attempted_1h;
        default:
            return 0;
    }
}

export function CapacityTab() {
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "fleet", "capacity"],
        queryFn: getFleetCapacity,
        refetchInterval: 30_000,
    });
    const [sort, setSort] = useState<{ by: string; desc: boolean }>({ by: "utilization", desc: true });

    const rows = useMemo(() => {
        const all = data?.data ?? [];
        return sort.by ? [...all].sort((a, b) => compare(a, b, sort.by) * (sort.desc ? -1 : 1)) : all;
    }, [data, sort]);

    const hot = rows.filter((r) => r.utilization > 0.8).length;
    const cold = rows.filter((r) => r.utilization < 0.5).length;

    return (
        <div>
            <div className="mb-4 flex flex-wrap items-start justify-between gap-x-8 gap-y-3">
                <p className="max-w-3xl text-[12.5px] leading-relaxed text-muted-foreground">
                    Placement spreads assigned mailboxes across live workers. The target is an operator-set
                    planning value, not a mailbox-provider send limit. New nodes fill gradually without
                    shrinking the displayed target. Send counters show succeeded / attempted during the
                    rolling last 60 minutes. “No attempts” is activity, not a capacity reading.
                </p>
                <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2">
                    {rows.length > 0 && (
                        <span className="flex items-center gap-3 text-xs text-muted-foreground tabular-nums">
                            <StatusDot tone={hot > 0 ? "danger" : "neutral"} className="text-xs">
                                {hot} hot
                            </StatusDot>
                            <StatusDot tone="neutral" className="text-xs">
                                {cold} cold
                            </StatusDot>
                            <span>of {rows.length}</span>
                        </span>
                    )}
                    <StateLegend label="Health states" entries={WORKER_HEALTH_LEGEND} />
                </div>
            </div>
            <DataTable
                columns={columns}
                rows={rows}
                getRowId={(w) => w.worker_id}
                loading={isLoading}
                error={error}
                onRetry={() => refetch()}
                errorTitle="Failed to load fleet capacity"
                sort={sort.by ? sort : undefined}
                onSortChange={setSort}
                storageKey="admin.fleet.capacity"
                csvName="warmbly-fleet-capacity"
                noun="workers"
                emptyTitle="No workers"
                emptyHint="Capacity rows appear once a worker has registered and heartbeated. Add one under Workers."
            />
        </div>
    );
}
