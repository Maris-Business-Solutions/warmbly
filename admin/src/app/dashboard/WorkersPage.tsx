// Fleet explorer: every machine running Warmbly, as a faceted browser.
//
// Both roles are here: a worker sends and syncs mail, a consumer processes
// events. They share one lifecycle (enrol, heartbeat, self-update) so they
// share one table. Data is fetched all-at-once (small N), so search, faceting
// and sort run client-side, mirroring the Users / Organizations browsers.

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { StatusBadge, StatusDot } from "@/components/ui/kit";
import {
    Explorer,
    FilterGroup,
    SearchFilter,
    SelectFilter,
    ToggleFilter,
    DateRangeFilter,
    NumberRangeFilter,
} from "@/components/data/Explorer";
import { DataTable, type Column } from "@/components/data/DataTable";
import { emptyRange, rangeActive, type DateRange } from "@/lib/dateRange";
import { listFleetNodes, nodeNeedsUpdate, nodeIsLive, type FleetNode } from "@/lib/api/client/admin/fleetNodes";
import { ResourceUsage } from "./fleet/ResourceUsage";
import { resourceCSV } from "./fleet/format";
import { TONE_TEXT, type Tone } from "@/lib/tones";

const OFFLINE_MS = 5 * 60_000;
const DAY_MS = 24 * 60 * 60 * 1000;

// Client-side date-range test for the workers explorer (data is in memory).
function inDateRange(iso: string | undefined, r: DateRange): boolean {
    if (!rangeActive(r)) return true;
    if (!iso) return false;
    const t = new Date(iso).getTime();
    if (r.preset && r.preset !== "custom") {
        return t >= Date.now() - Number(r.preset) * DAY_MS;
    }
    if (r.after && t < new Date(r.after).getTime()) return false;
    if (r.before && t >= new Date(r.before).getTime() + DAY_MS) return false;
    return true;
}

type LiveKey = "online" | "stale" | "offline" | "none";

function liveKey(w: FleetNode): LiveKey {
    if (!w.last_seen_at) return "none";
    const age = Date.now() - new Date(w.last_seen_at).getTime();
    if (age < 90_000) return "online";
    if (age < OFFLINE_MS * 2) return "stale";
    return "offline";
}

const LIVE_LABEL: Record<LiveKey, { label: string; tone: Tone }> = {
    online: { label: "online", tone: "success" },
    stale: { label: "stale", tone: "warning" },
    offline: { label: "offline", tone: "danger" },
    none: { label: "no heartbeat", tone: "neutral" },
};

const Empty = () => <span className="text-subtle-foreground">—</span>;

const columns: Column<FleetNode>[] = [
    {
        id: "name",
        header: "Worker",
        sortable: true,
        sortKey: "name",
        cell: (w) => (
            <div className="min-w-0">
                <Link
                    to={`/workers/${w.id}`}
                    onClick={(e) => e.stopPropagation()}
                    className="font-medium text-foreground hover:underline underline-offset-2"
                >
                    {w.name || w.id.slice(0, 8)}
                </Link>
                <div className="truncate font-mono text-[11px] text-subtle-foreground">{w.id}</div>
            </div>
        ),
        csv: (w) => w.name || w.id,
    },
    {
        id: "role",
        header: "Role",
        sortable: true,
        sortKey: "role",
        cell: (w) => <StatusBadge tone={w.role === "worker" ? "accent" : "strong"}>{w.role}</StatusBadge>,
        csv: (w) => w.role,
    },
    {
        id: "address",
        header: "Address",
        cell: (w) => (w.address ? <span className="font-mono text-xs text-muted-foreground">{w.address}</span> : <Empty />),
        csv: (w) => w.address,
    },
    {
        id: "region",
        header: "Region",
        cell: (w) => (w.region ? <span className="font-mono text-xs text-muted-foreground">{w.region}</span> : <Empty />),
        csv: (w) => w.region,
    },
    {
        id: "live",
        header: "Live",
        cell: (w) => {
            const l = LIVE_LABEL[liveKey(w)];
            return (
                <StatusDot tone={l.tone} pulse={l.tone === "success"} className="text-[12.5px] text-muted-foreground">
                    {l.label}
                </StatusDot>
            );
        },
        csv: (w) => LIVE_LABEL[liveKey(w)].label,
    },
    {
        id: "mailboxes",
        header: "Mailboxes",
        align: "right",
        sortable: true,
        sortKey: "mailboxes",
        cell: (w) =>
            w.mailbox_count === undefined ? (
                <Empty />
            ) : (
                <span className="tabular-nums">{w.mailbox_count}</span>
            ),
        csv: (w) => w.mailbox_count ?? "",
    },
    {
        // The pending update is shown inline, because "which machines are
        // behind" is the question this table exists to answer.
        id: "version",
        header: "Version",
        cell: (w) =>
            nodeNeedsUpdate(w) ? (
                <span className="font-mono text-xs whitespace-nowrap" title="Update pending">
                    {w.version || "—"}
                    <span className="text-subtle-foreground"> → </span>
                    <span className={TONE_TEXT.warning}>{w.desired_version}</span>
                </span>
            ) : (
                <span className="font-mono text-xs text-muted-foreground">{w.version || "—"}</span>
            ),
        csv: (w) => w.version || "",
    },
    {
        id: "memory",
        header: "Process RAM",
        align: "right",
        cell: (w) => <ResourceUsage usage={w.usage} kind="resident" live={nodeIsLive(w)} />,
        csv: (w) => nodeIsLive(w) ? w.usage?.resident_mb ?? "" : "stale",
        defaultHidden: true,
    },
    { id: "cpu", header: "CPU", align: "right", cell: (w) => <ResourceUsage usage={w.usage} kind="cpu" live={nodeIsLive(w)} />, csv: (w) => resourceCSV(w.usage, "cpu", nodeIsLive(w)), defaultHidden: true },
    { id: "ram", header: "RAM", align: "right", cell: (w) => <ResourceUsage usage={w.usage} kind="memory" live={nodeIsLive(w)} />, csv: (w) => resourceCSV(w.usage, "memory", nodeIsLive(w)), defaultHidden: true },
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
                <Empty />
            ),
        csv: (w) => (w.tags || []).join(" "),
        defaultHidden: true,
    },
    {
        id: "seen",
        header: "Last seen",
        sortable: true,
        sortKey: "seen",
        cell: (w) => (
            <span className="whitespace-nowrap text-muted-foreground tabular-nums">
                {w.last_seen_at ? new Date(w.last_seen_at).toLocaleString() : "—"}
            </span>
        ),
        csv: (w) => w.last_seen_at || "",
    },
    {
        id: "created",
        header: "Created",
        sortable: true,
        sortKey: "created",
        cell: (w) => (
            <span className="whitespace-nowrap text-muted-foreground tabular-nums">
                {new Date(w.created_at).toLocaleDateString()}
            </span>
        ),
        csv: (w) => w.created_at,
        defaultHidden: true,
    },
];

function compare(a: FleetNode, b: FleetNode, by: string): number {
    switch (by) {
        case "name":
            return (a.name || a.id).localeCompare(b.name || b.id);
        case "role":
            return a.role.localeCompare(b.role);
        case "mailboxes":
            return (a.mailbox_count ?? -1) - (b.mailbox_count ?? -1);
        case "seen":
            return new Date(a.last_seen_at || 0).getTime() - new Date(b.last_seen_at || 0).getTime();
        case "created":
            return new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
        default:
            return 0;
    }
}

export default function WorkersPage() {
    const nav = useNavigate();
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "workers", "managed"],
        queryFn: () => listFleetNodes(),
    });

    const [query, setQuery] = useState("");
    const [live, setLive] = useState("");
    const [activeOnly, setActiveOnly] = useState(false);
    const [hasMailboxes, setHasMailboxes] = useState(false);
    const [hasError, setHasError] = useState(false);
    const [hasTags, setHasTags] = useState(false);
    const [mbMin, setMbMin] = useState<number | undefined>();
    const [mbMax, setMbMax] = useState<number | undefined>();
    const [created, setCreated] = useState<DateRange>(emptyRange);
    const [lastSeen, setLastSeen] = useState<DateRange>(emptyRange);
    const [sort, setSort] = useState<{ by: string; desc: boolean }>({ by: "", desc: true });

    const rows = useMemo(() => {
        let all = data?.data ?? [];
        const q = query.trim().toLowerCase();
        if (q) {
            all = all.filter((w) =>
                `${w.name} ${w.id} ${w.role} ${w.address} ${w.region} ${w.version} ${(w.tags || []).join(" ")}`
                    .toLowerCase()
                    .includes(q),
            );
        }
        if (live) all = all.filter((w) => liveKey(w) === live);
        if (activeOnly) all = all.filter((w) => w.active);
        if (hasMailboxes) all = all.filter((w) => (w.mailbox_count ?? 0) > 0);
        if (hasError) all = all.filter((w) => !!w.last_error && w.last_error !== "");
        if (hasTags) all = all.filter((w) => (w.tags?.length ?? 0) > 0);
        if (mbMin !== undefined) all = all.filter((w) => (w.mailbox_count ?? 0) >= mbMin);
        if (mbMax !== undefined) all = all.filter((w) => (w.mailbox_count ?? 0) <= mbMax);
        if (rangeActive(created)) all = all.filter((w) => inDateRange(w.created_at, created));
        if (rangeActive(lastSeen)) all = all.filter((w) => inDateRange(w.last_seen_at ?? undefined, lastSeen));
        if (sort.by) {
            all = [...all].sort((a, b) => compare(a, b, sort.by) * (sort.desc ? -1 : 1));
        }
        return all;
    }, [data, query, live, activeOnly, hasMailboxes, hasError, hasTags, mbMin, mbMax, created, lastSeen, sort]);

    const activeCount =
        (query ? 1 : 0) +
        (live ? 1 : 0) +
        (activeOnly ? 1 : 0) +
        (hasMailboxes ? 1 : 0) +
        (hasError ? 1 : 0) +
        (hasTags ? 1 : 0) +
        (mbMin !== undefined || mbMax !== undefined ? 1 : 0) +
        [created, lastSeen].filter(rangeActive).length +
        (sort.by ? 1 : 0);

    function resetAll() {
        setQuery("");
        setLive("");
        setActiveOnly(false);
        setHasMailboxes(false);
        setHasError(false);
        setHasTags(false);
        setMbMin(undefined);
        setMbMax(undefined);
        setCreated(emptyRange);
        setLastSeen(emptyRange);
        setSort({ by: "", desc: true });
    }

    return (
        <div>
            <PageHeader
                title="Fleet"
                meta={
                    data?.data ? (
                        <span className="text-[12.5px] text-muted-foreground tabular-nums">{data.data.length}</span>
                    ) : undefined
                }
                description="Every machine running Warmbly. They enrol themselves, report what they are running, and stay on the version you set."
            >
                <Button size="sm" asChild>
                    <Link to="/workers/new">
                        <Plus className="size-3.5" />
                        Add worker
                    </Link>
                </Button>
            </PageHeader>

            <Explorer
                activeCount={activeCount}
                onReset={resetAll}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="Name, host, IP, version, tag…" />
                        </FilterGroup>
                        <FilterGroup label="Liveness">
                            <SelectFilter
                                value={live || "any"}
                                onChange={(v) => setLive(v === "any" ? "" : v)}
                                options={[
                                    { value: "any", label: "Any" },
                                    { value: "online", label: "Online" },
                                    { value: "stale", label: "Stale" },
                                    { value: "offline", label: "Offline" },
                                    { value: "none", label: "No heartbeat" },
                                ]}
                            />
                        </FilterGroup>
                        <FilterGroup label="Flags">
                            <div className="flex flex-col gap-2">
                                <ToggleFilter checked={activeOnly} onChange={setActiveOnly} label="Active only" />
                                <ToggleFilter checked={hasMailboxes} onChange={setHasMailboxes} label="Has mailboxes" />
                                <ToggleFilter checked={hasError} onChange={setHasError} label="Has node error" />
                                <ToggleFilter checked={hasTags} onChange={setHasTags} label="Has tags" />
                            </div>
                        </FilterGroup>
                        <FilterGroup label="Mailbox count">
                            <NumberRangeFilter min={mbMin} max={mbMax} onMinChange={setMbMin} onMaxChange={setMbMax} />
                        </FilterGroup>
                        <FilterGroup label="Connected">
                            <DateRangeFilter value={created} onChange={setCreated} />
                        </FilterGroup>
                        <FilterGroup label="Last seen">
                            <DateRangeFilter value={lastSeen} onChange={setLastSeen} mode="custom" />
                        </FilterGroup>
                    </>
                }
            >
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(w) => w.id}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load workers"
                    onRowClick={(w) => nav(`/workers/${w.id}`)}
                    sort={sort.by ? sort : undefined}
                    onSortChange={setSort}
                    storageKey="admin.workers"
                    csvName="warmbly-workers"
                    noun="workers"
                    emptyTitle="No workers"
                    emptyHint="No workers match these filters."
                />
            </Explorer>
        </div>
    );
}
