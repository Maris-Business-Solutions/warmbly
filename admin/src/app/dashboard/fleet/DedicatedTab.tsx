// Dedicated bindings: which worker is reserved for which workspace. Keyed
// under ["admin","workers"] so the realtime workers spine refreshes it; no poll.

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { Plus, Unlink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useConfirm } from "@/components/ConfirmDialog";
import {
    listDedicatedAssignments,
    releaseDedicatedWorker,
    type AdminDedicatedAssignment,
} from "@/lib/api/client/admin/fleet";
import { LiveDot } from "./tones";
import { fmtDate, shortId } from "./format";
import { ConvertDedicatedDialog } from "./ConvertDedicatedDialog";

export function DedicatedTab() {
    const qc = useQueryClient();
    const confirm = useConfirm();
    const [convertOpen, setConvertOpen] = useState(false);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "workers", "dedicated"],
        queryFn: listDedicatedAssignments,
    });

    const release = useMutation({
        mutationFn: (orgId: string) => releaseDedicatedWorker(orgId),
        onSuccess: (res) => {
            toast.success(
                `Reservation released; ${res.accounts_remaining} mailbox${res.accounts_remaining === 1 ? "" : "es"} still on that worker`,
            );
            qc.invalidateQueries({ queryKey: ["admin", "workers"] });
            qc.invalidateQueries({ queryKey: ["admin", "fleet"] });
        },
        onError: (e: Error) => toast.error(e.message || "Release failed"),
    });

    async function onRelease(a: AdminDedicatedAssignment) {
        const ok = await confirm({
            title: `Release ${a.worker_name || shortId(a.worker_id)} from ${a.organization_name}?`,
            description: `The worker rejoins the general fleet. The workspace's ${a.account_count} mailbox${a.account_count === 1 ? "" : "es"} stay where they are: moving one changes the address its provider sees signing in, so the rotation loop only re-places them when it has a reason to.`,
            confirmLabel: "Release",
            destructive: true,
        });
        if (ok) release.mutate(a.organization_id);
    }

    const columns: Column<AdminDedicatedAssignment>[] = [
        {
            id: "worker",
            header: "Worker",
            cell: (a) => (
                <div className="min-w-0">
                    <Link
                        to={`/workers/${a.worker_id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="font-medium text-foreground underline-offset-2 hover:underline"
                    >
                        {a.worker_name || shortId(a.worker_id)}
                    </Link>
                    <div className="truncate font-mono text-[11px] text-subtle-foreground">{a.worker_id}</div>
                </div>
            ),
            csv: (a) => a.worker_name || a.worker_id,
        },
        { id: "live", header: "Live", cell: (a) => <LiveDot live={a.worker_live} />, csv: (a) => (a.worker_live ? "live" : "offline") },
        {
            id: "org",
            header: "Workspace",
            cell: (a) => (
                <Link
                    to={`/organizations/${a.organization_id}`}
                    onClick={(e) => e.stopPropagation()}
                    className="text-foreground underline-offset-2 hover:underline"
                >
                    {a.organization_name || shortId(a.organization_id)}
                </Link>
            ),
            csv: (a) => a.organization_name,
        },
        {
            id: "sub",
            header: "Subscription",
            cell: (a) => (
                <span className="font-mono text-xs text-muted-foreground" title={a.subscription_id}>
                    {shortId(a.subscription_id)}
                </span>
            ),
            csv: (a) => a.subscription_id,
        },
        { id: "assigned", header: "Assigned", cell: (a) => <span className="whitespace-nowrap text-muted-foreground tabular-nums">{fmtDate(a.assigned_at)}</span>, csv: (a) => a.assigned_at },
        { id: "accounts", header: "Accounts", align: "right", cell: (a) => <span className="tabular-nums">{a.account_count}</span>, csv: (a) => a.account_count },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (a) => (
                <Button
                    size="xs"
                    variant="ghost"
                    className="text-muted-foreground hover:text-foreground"
                    onClick={(e) => {
                        e.stopPropagation();
                        void onRelease(a);
                    }}
                    disabled={release.isPending}
                >
                    <Unlink className="size-3" />
                    Release
                </Button>
            ),
        },
    ];

    return (
        <div>
            <div className="mb-4 flex flex-wrap items-start justify-between gap-x-8 gap-y-3">
                <p className="max-w-3xl text-[12.5px] leading-relaxed text-muted-foreground">
                    A reserved worker carries one workspace's mailboxes and nothing else, so that workspace always
                    authenticates to its mailbox providers from an address no other tenant sends from. That is what
                    the entitlement buys: fewer sign-in challenges and no shared per-IP auth throttle. It is a strong
                    placement preference, not a pin, so a worker going down never strands the workspace.
                </p>
                <Button size="sm" className="shrink-0" onClick={() => setConvertOpen(true)}>
                    <Plus className="size-3.5" />
                    Reserve a worker
                </Button>
            </div>
            <DataTable
                columns={columns}
                rows={data?.data ?? []}
                getRowId={(a) => a.id}
                loading={isLoading}
                error={error}
                onRetry={() => refetch()}
                errorTitle="Failed to load dedicated assignments"
                storageKey="admin.fleet.dedicated"
                csvName="warmbly-dedicated-workers"
                noun="assignments"
                emptyTitle="No reserved workers"
                emptyHint="Every worker is shared by the whole fleet. Reserve one above to give a workspace its own sending address."
            />
            <ConvertDedicatedDialog open={convertOpen} onOpenChange={setConvertOpen} />
        </div>
    );
}
