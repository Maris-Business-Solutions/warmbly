// Customer webhook delivery health across the instance, and the endpoints
// that are currently failing. Polls at 30s; no realtime event covers the
// delivery queue.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AlertTriangle, CheckCircle2, Clock, RotateCcw, Trash2, XCircle, Ban } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Section, Stat, StatGrid, StatusBadge, StatusDot } from "@/components/ui/kit";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useConfirm } from "@/components/ConfirmDialog";
import { getWebhookHealth, reclaimWebhookDeliveries, type AdminWebhookEndpointRow } from "@/lib/api/client/admin/sends";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { ExpandableText } from "@/app/dashboard/jobs/ExpandableText";
import { absolute, relative } from "@/app/dashboard/jobs/format";
import { TabIntro, WorkspaceLink } from "@/app/dashboard/sends/shared";
import { fmt } from "@/app/dashboard/sends/status";

const columns: Column<AdminWebhookEndpointRow>[] = [
    {
        id: "workspace",
        header: "Workspace",
        cell: (r) => <WorkspaceLink id={r.organization_id} name={r.organization_name} />,
        csv: (r) => r.organization_name,
    },
    {
        id: "url",
        header: "Endpoint",
        className: "max-w-sm",
        cell: (r) => (
            <div className="min-w-0">
                <div className="truncate font-mono text-[11.5px] text-foreground" title={r.url}>
                    {r.url}
                </div>
                {r.description && <div className="truncate text-xs text-muted-foreground">{r.description}</div>}
            </div>
        ),
        csv: (r) => r.url,
    },
    {
        id: "consecutive",
        header: "Consecutive failures",
        align: "right",
        cell: (r) => (
            <span
                className={cn(
                    "text-xs tabular-nums",
                    r.consecutive_failures > 0 ? cn("font-medium", TONE_TEXT.danger) : "text-muted-foreground",
                )}
            >
                {r.consecutive_failures.toLocaleString()}
            </span>
        ),
        csv: (r) => r.consecutive_failures,
    },
    {
        id: "last_failure",
        header: "Last failure",
        className: "max-w-md py-2",
        cell: (r) => (
            <div className="min-w-0">
                <ExpandableText text={r.last_failure_reason} mono />
                {r.last_failure_at && (
                    <div className="text-xs text-muted-foreground tabular-nums" title={absolute(r.last_failure_at)}>
                        {relative(r.last_failure_at)}
                        {r.last_success_at && ` · last success ${relative(r.last_success_at)}`}
                    </div>
                )}
            </div>
        ),
        csv: (r) => r.last_failure_reason,
    },
    {
        id: "week",
        header: "Last 7 days",
        align: "right",
        cell: (r) => (
            <span className="whitespace-nowrap text-xs tabular-nums text-subtle-foreground" title="delivered / failed / dropped">
                <span className={TONE_TEXT.success}>{r.deliveries_last_7d.toLocaleString()}</span>
                {" / "}
                <span className={r.failed_last_7d > 0 ? TONE_TEXT.danger : "text-muted-foreground"}>
                    {r.failed_last_7d.toLocaleString()}
                </span>
                {" / "}
                <span className={r.drops_last_7d > 0 ? TONE_TEXT.warning : "text-muted-foreground"}>
                    {r.drops_last_7d.toLocaleString()}
                </span>
            </span>
        ),
        csv: (r) => `${r.deliveries_last_7d}/${r.failed_last_7d}/${r.drops_last_7d}`,
    },
    {
        id: "enabled",
        header: "Enabled",
        cell: (r) =>
            r.enabled ? (
                <StatusDot tone="success" className="text-xs text-muted-foreground">
                    enabled
                </StatusDot>
            ) : (
                <StatusBadge tone="neutral">disabled</StatusBadge>
            ),
        csv: (r) => (r.enabled ? "yes" : "no"),
    },
];

export function WebhooksTab() {
    const qc = useQueryClient();
    const confirm = useConfirm();

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "sends", "webhooks"],
        queryFn: getWebhookHealth,
        refetchInterval: 30_000,
    });

    const reclaim = useMutation({
        mutationFn: reclaimWebhookDeliveries,
        onSuccess: (res) => {
            toast.success(`Reclaimed ${res.reclaimed.toLocaleString()} stuck ${res.reclaimed === 1 ? "delivery" : "deliveries"}`);
            qc.invalidateQueries({ queryKey: ["admin", "sends", "webhooks"] });
        },
        onError: (err: Error) => toast.error(err.message || "Failed to reclaim deliveries"),
    });

    async function onReclaim() {
        const ok = await confirm({
            title: "Reclaim stuck deliveries?",
            description: `${
                data
                    ? `Deliveries claimed longer ago than the ${data.lease_minutes} minute lease`
                    : "Deliveries claimed longer ago than the lease window"
            } are handed back to the queue so a delivery worker can pick them up again. An endpoint may receive a duplicate if the original attempt did complete after its lease expired.`,
            confirmLabel: "Reclaim",
        });
        if (!ok) return;
        reclaim.mutate();
    }

    const rows = data?.failing_endpoints ?? [];

    return (
        <div>
            <TabIntro
                actions={
                    <Button size="sm" variant="outline" onClick={onReclaim} disabled={reclaim.isPending}>
                        <RotateCcw />
                        {reclaim.isPending ? "Reclaiming…" : "Reclaim stuck deliveries"}
                    </Button>
                }
            >
                Instance-wide delivery of customer webhooks.
                {data && <> A delivery is stale once it has been claimed for more than {data.lease_minutes} minutes without a result.</>}
            </TabIntro>

            <StatGrid className="md:grid-cols-3 xl:grid-cols-6">
                <Stat
                    icon={AlertTriangle}
                    label="Stale in flight"
                    value={fmt(data?.in_flight_stale)}
                    loading={isLoading}
                    tone={data?.in_flight_stale ? "warning" : undefined}
                    sub={data ? `lease ${data.lease_minutes} min` : undefined}
                />
                <Stat icon={Clock} label="Pending due" value={fmt(data?.pending_due)} loading={isLoading} />
                <Stat icon={CheckCircle2} label="Delivered 24h" value={fmt(data?.delivered_last_24h)} loading={isLoading} />
                <Stat
                    icon={XCircle}
                    label="Failed 24h"
                    value={fmt(data?.failed_last_24h)}
                    loading={isLoading}
                    tone={data?.failed_last_24h ? "warning" : undefined}
                />
                <Stat
                    icon={Ban}
                    label="Abandoned 24h"
                    value={fmt(data?.abandoned_last_24h)}
                    loading={isLoading}
                    tone={data?.abandoned_last_24h ? "danger" : undefined}
                />
                <Stat
                    icon={Trash2}
                    label="Drops 7d"
                    value={fmt(data?.drops_last_7d)}
                    loading={isLoading}
                    sub="queue full, never attempted"
                />
            </StatGrid>

            <Section title="Failing endpoints" description="Endpoints failing consecutively, worst first.">
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(r) => r.id}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load webhook health"
                    storageKey="admin.sends.webhooks"
                    csvName="warmbly-failing-webhooks"
                    noun="endpoints"
                    emptyTitle="No failing endpoints"
                    emptyHint="Every customer endpoint accepted its last delivery. Rows appear when an endpoint fails consecutively, worst first."
                />
            </Section>
        </div>
    );
}
