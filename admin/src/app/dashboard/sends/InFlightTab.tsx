// Reserved campaign sends no worker result has resolved. A row lives between
// ReserveSend and the EMAIL_SENT / EMAIL_FAILED answer; the consumer's stuck
// send reclaimer resolves anything older than the reclaim window. No realtime
// event covers reservations, so this polls at 30s.

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { AlertTriangle, Clock, Hourglass, Play, Send, Timer } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Section, Stat, StatGrid, StatusBadge } from "@/components/ui/kit";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useConfirm } from "@/components/ConfirmDialog";
import { listInFlightSends, type AdminInFlightSend } from "@/lib/api/client/admin/sends";
import { runJob, STUCK_SEND_RECLAIMER_JOB } from "@/lib/api/client/admin/jobs";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { absolute, humanSeconds, relative, shortId } from "@/app/dashboard/jobs/format";
import { TabIntro, WorkspaceLink } from "@/app/dashboard/sends/shared";
import { fmt, TASK_TONE } from "@/app/dashboard/sends/status";

export function InFlightTab() {
    const qc = useQueryClient();
    const confirm = useConfirm();

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "sends", "in-flight"],
        queryFn: () => listInFlightSends(200),
        refetchInterval: 30_000,
    });

    const run = useMutation({
        mutationFn: () => runJob(STUCK_SEND_RECLAIMER_JOB),
        onSuccess: () => {
            toast.success("Reclaimer requested; the consumer's reclaim loop runs it at its next poll, within about 15 seconds");
            qc.invalidateQueries({ queryKey: ["admin", "jobs"] });
        },
        onError: (err: Error) => toast.error(err.message || "Failed to request the reclaimer"),
    });

    const summary = data?.summary;
    const reclaimAfter = summary?.reclaim_after_minutes;
    const windowSeconds = (reclaimAfter ?? 0) * 60;

    async function onRun() {
        const ok = await confirm({
            title: "Run the stuck-send reclaimer now?",
            description: `This asks the consumer's reclaim loop to run at its next poll, within about 15 seconds. Every reservation older than ${reclaimAfter ?? "the reclaim window in"} minutes is resolved: a task that already carries a Message-ID is stamped as sent, anything else is walked back as a failed attempt and retried on the next routing tick.`,
            confirmLabel: "Run reclaimer",
        });
        if (!ok) return;
        run.mutate();
    }

    const rows = data?.data ?? [];

    const columns: Column<AdminInFlightSend>[] = [
        {
            id: "campaign",
            header: "Campaign",
            cell: (r) => (
                <div className="min-w-0">
                    <Link
                        to={`/campaigns/${r.campaign_id}`}
                        className="text-[13px] font-medium text-foreground decoration-border-strong underline-offset-2 hover:underline"
                    >
                        {r.campaign_name || shortId(r.campaign_id)}
                    </Link>
                    <div className="font-mono text-[11px] text-subtle-foreground">{shortId(r.campaign_id)}</div>
                </div>
            ),
            csv: (r) => r.campaign_name,
        },
        {
            id: "workspace",
            header: "Workspace",
            cell: (r) => <WorkspaceLink id={r.organization_id} name={r.organization_name} />,
            csv: (r) => r.organization_name || "",
        },
        {
            id: "contact",
            header: "Contact",
            cell: (r) => <span className="text-[13px] text-foreground">{r.contact_email}</span>,
            csv: (r) => r.contact_email,
        },
        {
            id: "mailbox",
            header: "Mailbox",
            cell: (r) => (
                <div className="min-w-0">
                    <div className="truncate text-[13px] text-foreground">{r.mailbox_email || "—"}</div>
                    {r.worker_id && (
                        <Link
                            to={`/workers/${r.worker_id}`}
                            className="font-mono text-[11px] text-subtle-foreground hover:text-muted-foreground hover:underline"
                        >
                            worker {shortId(r.worker_id)}
                        </Link>
                    )}
                </div>
            ),
            csv: (r) => r.mailbox_email,
        },
        {
            id: "task",
            header: "Task",
            cell: (r) => (
                <div className="flex items-center gap-2">
                    <StatusBadge tone={TASK_TONE[r.task_status] ?? "neutral"} dot>
                        {r.task_status || "unknown"}
                    </StatusBadge>
                    {r.task_id && <span className="font-mono text-[11px] text-subtle-foreground">{shortId(r.task_id)}</span>}
                </div>
            ),
            csv: (r) => r.task_status,
        },
        {
            id: "message_id",
            header: "Message ID",
            cell: (r) =>
                r.has_message_id ? (
                    <StatusBadge
                        tone="success"
                        title="The worker put the mail on the wire and only the stamp was lost; the reclaimer stamps it rather than retry"
                    >
                        on the wire
                    </StatusBadge>
                ) : (
                    <span className="text-xs text-subtle-foreground">none</span>
                ),
            csv: (r) => (r.has_message_id ? "yes" : "no"),
        },
        {
            id: "dispatched",
            header: "Dispatched",
            align: "right",
            cell: (r) => {
                const late = windowSeconds > 0 && r.age_seconds >= windowSeconds;
                return (
                    <span
                        className={cn(
                            "whitespace-nowrap text-xs tabular-nums",
                            late ? cn("font-medium", TONE_TEXT.danger) : "text-muted-foreground",
                        )}
                        title={absolute(r.dispatched_at)}
                    >
                        {humanSeconds(r.age_seconds)} ago
                    </span>
                );
            },
            csv: (r) => r.dispatched_at,
        },
    ];

    return (
        <div>
            <TabIntro
                actions={
                    <Button size="sm" variant="outline" onClick={onRun} disabled={run.isPending}>
                        <Play />
                        {run.isPending ? "Requesting…" : "Run reclaimer now"}
                    </Button>
                }
            >
                A reservation is written before SEND_EMAIL goes on the bus and resolved by exactly one worker result.
                {reclaimAfter !== undefined && (
                    <> The reclaimer sweeps every 5 minutes and resolves anything older than {reclaimAfter} minutes.</>
                )}
            </TabIntro>

            <StatGrid className="md:grid-cols-5">
                <Stat icon={Send} label="In flight" value={fmt(summary?.total)} loading={isLoading} />
                <Stat icon={Timer} label="Under 5 min" value={fmt(summary?.under_5m)} loading={isLoading} />
                <Stat icon={Clock} label="Under 30 min" value={fmt(summary?.under_30m)} loading={isLoading} />
                <Stat
                    icon={AlertTriangle}
                    label="Past reclaim window"
                    value={fmt(summary?.past_reclaim_window)}
                    loading={isLoading}
                    tone={summary?.past_reclaim_window ? "danger" : undefined}
                    sub={reclaimAfter !== undefined ? `older than ${reclaimAfter} min` : undefined}
                />
                <Stat
                    icon={Hourglass}
                    label="Oldest"
                    value={summary?.oldest_dispatched_at ? relative(summary.oldest_dispatched_at) : "—"}
                    loading={isLoading}
                    sub={summary?.oldest_dispatched_at ? absolute(summary.oldest_dispatched_at) : undefined}
                    className="col-span-2 md:col-span-1"
                />
            </StatGrid>

            <Section title="Reservations">
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(r) => `${r.campaign_id}:${r.contact_id}:${r.sequence_id}`}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load in-flight sends"
                    storageKey="admin.sends.in-flight"
                    csvName="warmbly-in-flight-sends"
                    noun="sends"
                    emptyTitle="Nothing in flight"
                    emptyHint="No reserved send is waiting on a worker result. Rows appear between a SEND_EMAIL dispatch and its EMAIL_SENT or EMAIL_FAILED answer."
                />
            </Section>
        </div>
    );
}
