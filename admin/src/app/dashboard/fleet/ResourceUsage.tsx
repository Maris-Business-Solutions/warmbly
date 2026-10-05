import type { NodeUsage } from "@/lib/api/client/admin/fleetNodes";

export function ResourceUsage({ usage, kind, live }: { usage?: NodeUsage; kind: "cpu" | "memory" | "resident"; live: boolean }) {
    if (!live) return <span className="text-subtle-foreground" title="The last heartbeat is stale or the node is stopped">Stale</span>;
    if (kind === "resident") {
        return <span className="tabular-nums" title="Resident memory of the Warmbly process, not Go heap allocation">{usage?.resident_mb == null ? "Unavailable" : `${usage.resident_mb.toLocaleString()} MiB`}</span>;
    }
    const scope = kind === "cpu" ? usage?.cpu_scope : usage?.memory_scope;
    const used = usage?.memory_used_mb;
    const limit = usage?.memory_limit_mb;
    const percent = kind === "cpu" ? usage?.cpu_percent : used != null && limit != null && limit > 0 ? used / limit * 100 : undefined;
    if (percent == null || !scope) return <span className="text-subtle-foreground">Unavailable</span>;
    return (
        <div className="whitespace-nowrap tabular-nums" title={kind === "cpu" ? "Average CPU use since the previous heartbeat, normalized to available cores" : "Container memory against its limit, or host memory excluding reclaimable cache"}>
            <span>{percent.toFixed(1)}%</span>
            <span className="ml-1 text-[11px] text-subtle-foreground">{scope}</span>
            {kind === "memory" && <div className="text-[11px] text-muted-foreground">{used?.toLocaleString()} / {limit?.toLocaleString()} MiB</div>}
        </div>
    );
}
