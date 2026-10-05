// Recent task failures: what a task reported when it stopped, with the
// mailbox and workspace it belongs to. Polls at 60s; there is no event.

import { useQuery } from "@tanstack/react-query";
import { StatusBadge } from "@/components/ui/kit";
import { DataTable, type Column } from "@/components/data/DataTable";
import { listTaskFailures, type AdminTaskFailureRow } from "@/lib/api/client/admin/sends";
import { ExpandableText } from "@/app/dashboard/jobs/ExpandableText";
import { absolute, relative, shortId } from "@/app/dashboard/jobs/format";
import { TabIntro, WorkspaceLink } from "@/app/dashboard/sends/shared";
import { TASK_TONE } from "@/app/dashboard/sends/status";

const columns: Column<AdminTaskFailureRow>[] = [
    {
        id: "title",
        header: "Failure",
        className: "max-w-md py-2",
        cell: (r) => (
            <div className="min-w-0">
                <div className="text-[13px] font-medium text-foreground">{r.title || "Task failed"}</div>
                <ExpandableText text={r.message} className="text-muted-foreground" />
            </div>
        ),
        csv: (r) => `${r.title}: ${r.message}`,
    },
    {
        id: "task",
        header: "Task",
        cell: (r) => (
            <div className="flex items-center gap-2">
                <span className="font-mono text-xs text-foreground">{r.task_type}</span>
                <StatusBadge tone={TASK_TONE[r.task_status] ?? "neutral"} dot>
                    {r.task_status}
                </StatusBadge>
            </div>
        ),
        csv: (r) => `${r.task_type} (${r.task_status})`,
    },
    {
        id: "mailbox",
        header: "Mailbox",
        cell: (r) => (
            <div className="min-w-0">
                <div className="truncate text-[13px] text-foreground">{r.mailbox_email || "—"}</div>
                <div className="font-mono text-[11px] text-subtle-foreground">{shortId(r.email_account_id)}</div>
            </div>
        ),
        csv: (r) => r.mailbox_email,
    },
    {
        id: "workspace",
        header: "Workspace",
        cell: (r) => <WorkspaceLink id={r.organization_id} name={r.organization_name} />,
        csv: (r) => r.organization_name || "",
    },
    {
        id: "occurred",
        header: "Occurred",
        align: "right",
        cell: (r) => (
            <span className="whitespace-nowrap text-xs text-muted-foreground tabular-nums" title={absolute(r.occurred_at)}>
                {relative(r.occurred_at)}
            </span>
        ),
        csv: (r) => r.occurred_at,
    },
];

export function FailuresTab() {
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "sends", "failures"],
        queryFn: () => listTaskFailures(100),
        refetchInterval: 60_000,
    });
    const rows = data?.data ?? [];

    return (
        <div>
            <TabIntro>
                The most recent failures tasks recorded about themselves: auth errors, provider refusals, send exceptions. Newest first.
            </TabIntro>
            <DataTable
                columns={columns}
                rows={rows}
                getRowId={(r) => `${r.task_id}:${r.occurred_at}`}
                loading={isLoading}
                error={error}
                onRetry={() => refetch()}
                errorTitle="Failed to load task failures"
                storageKey="admin.sends.failures"
                csvName="warmbly-task-failures"
                noun="failures"
                emptyTitle="No recent failures"
                emptyHint="No task has recorded a failure recently. Rows appear when a send, sync or warmup task stops with an error."
            />
        </div>
    );
}
