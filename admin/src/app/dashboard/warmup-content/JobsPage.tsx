// /warmup-content/jobs: paged table of generation jobs (sync + batch),
// polled live so running jobs update without a manual refresh. Batch jobs
// surface their OpenAI batch status and an inline Cancel action.

import { useMemo, useState } from "react";
import {
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";
import { Ban, CircleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Callout, Property, PropertyList, StatusBadge } from "@/components/ui/kit";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { ErrorState } from "@/components/ErrorState";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DataTable, type Column } from "@/components/data/DataTable";
import { StateLegend } from "@/components/StateLegend";
import { GENERATION_JOB_LEGEND } from "@/lib/legends";
import { useCursorPager } from "@/lib/useCursorPager";
import {
    cancelWarmupBatch,
    getWarmupGenerationJob,
    isJobActive,
    isJobCancellable,
    listWarmupGenerationJobs,
    type WarmupGenerationJob,
} from "@/lib/api/client/admin/warmupContent";
import { ModeBadge, PoolBadge } from "./components";
import { batchTone, fmtDate, jobTone } from "./shared";

export default function JobsPage() {
    const qc = useQueryClient();
    const pager = useCursorPager();
    const [openId, setOpenId] = useState<string | null>(null);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup-content", "jobs", pager.cursor],
        queryFn: () => listWarmupGenerationJobs({ cursor: pager.cursor, limit: 50 }),
        placeholderData: keepPreviousData,
        // Poll only while at least one job is still in flight; idle pages stop
        // hammering the endpoint.
        refetchInterval: (query) => {
            const rows = query.state.data?.data ?? [];
            return rows.some(isJobActive) ? 10_000 : false;
        },
    });

    const cancel = useMutation({
        mutationFn: (id: string) => cancelWarmupBatch(id),
        onSuccess: () => {
            toast.success("Batch cancellation requested");
            qc.invalidateQueries({ queryKey: ["admin", "warmup-content", "jobs"] });
        },
        onError: (err: Error) => toast.error(err.message || "Failed to cancel job"),
    });

    const columns: Column<WarmupGenerationJob>[] = useMemo(
        () => [
            {
                id: "status",
                header: "Status",
                cell: (j) => (
                    <StatusBadge tone={jobTone(j.status)} dot>
                        {j.status}
                    </StatusBadge>
                ),
                csv: (j) => j.status,
            },
            {
                id: "mode",
                header: "Mode",
                cell: (j) => <ModeBadge mode={j.mode} />,
                csv: (j) => j.mode ?? "sync",
            },
            {
                id: "batch_status",
                header: "Batch",
                cell: (j) =>
                    j.mode === "batch" && j.batch_status ? (
                        <StatusBadge tone={batchTone(j.batch_status)}>{j.batch_status}</StatusBadge>
                    ) : (
                        <span className="text-subtle-foreground">—</span>
                    ),
                csv: (j) => j.batch_status ?? "",
            },
            {
                id: "pool",
                header: "Pool",
                cell: (j) => <PoolBadge pool={j.pool_type} />,
                csv: (j) => j.pool_type,
            },
            {
                id: "segment",
                header: "Segment",
                cell: (j) => <span>{j.segment || "—"}</span>,
                csv: (j) => j.segment,
            },
            {
                id: "trigger",
                header: "Trigger",
                cell: (j) => (
                    <span className="text-muted-foreground">{j.trigger || "—"}</span>
                ),
                csv: (j) => j.trigger,
            },
            {
                id: "model",
                header: "Model",
                cell: (j) => <span className="whitespace-nowrap">{j.model || "—"}</span>,
                csv: (j) => j.model,
            },
            {
                id: "counts",
                header: "Generated / Requested",
                align: "right",
                cell: (j) => (
                    <span className="whitespace-nowrap tabular-nums">
                        {j.generated_count}
                        <span className="text-muted-foreground"> / {j.requested_count}</span>
                    </span>
                ),
                csv: (j) => `${j.generated_count}/${j.requested_count}`,
            },
            {
                id: "rejected",
                header: "Lint rej.",
                align: "right",
                cell: (j) => (
                    <span
                        className={cn(
                            "tabular-nums",
                            j.lint_rejected_count > 0 ? TONE_TEXT.warning : "text-muted-foreground",
                        )}
                    >
                        {j.lint_rejected_count}
                    </span>
                ),
                csv: (j) => j.lint_rejected_count,
            },
            {
                id: "failed",
                header: "Failed",
                align: "right",
                cell: (j) => (
                    <span
                        className={cn(
                            "tabular-nums",
                            j.failed_count > 0 ? TONE_TEXT.danger : "text-muted-foreground",
                        )}
                    >
                        {j.failed_count}
                    </span>
                ),
                csv: (j) => j.failed_count,
            },
            {
                id: "window",
                header: "Window",
                cell: (j) => (
                    <span className="text-muted-foreground">{j.completion_window || "—"}</span>
                ),
                csv: (j) => j.completion_window ?? "",
                defaultHidden: true,
            },
            {
                id: "batch_id",
                header: "Batch ID",
                cell: (j) => (
                    <span className="font-mono text-xs text-muted-foreground">
                        {j.batch_id || "—"}
                    </span>
                ),
                csv: (j) => j.batch_id ?? "",
                defaultHidden: true,
            },
            {
                id: "started",
                header: "Started",
                cell: (j) => (
                    <span className="whitespace-nowrap text-muted-foreground">
                        {fmtDate(j.started_at)}
                    </span>
                ),
                csv: (j) => j.started_at ?? "",
            },
            {
                id: "finished",
                header: "Finished",
                cell: (j) => (
                    <span className="whitespace-nowrap text-muted-foreground">
                        {fmtDate(j.finished_at)}
                    </span>
                ),
                csv: (j) => j.finished_at ?? "",
            },
            {
                id: "error",
                header: "Error",
                cell: (j) =>
                    j.error ? (
                        <span className={TONE_TEXT.danger} title={j.error}>
                            {j.error.length > 60 ? `${j.error.slice(0, 60)}…` : j.error}
                        </span>
                    ) : (
                        <span className="text-subtle-foreground">—</span>
                    ),
                csv: (j) => j.error,
                defaultHidden: true,
            },
            {
                id: "created",
                header: "Created",
                cell: (j) => (
                    <span className="whitespace-nowrap text-muted-foreground">
                        {new Date(j.created_at).toLocaleString()}
                    </span>
                ),
                csv: (j) => j.created_at,
                defaultHidden: true,
            },
            {
                id: "actions",
                header: "Actions",
                align: "right",
                cell: (j) =>
                    isJobCancellable(j) ? (
                        <div
                            className="flex justify-end"
                            onClick={(e) => e.stopPropagation()}
                        >
                            <Button
                                size="xs"
                                variant="ghost"
                                className={cn(TONE_TEXT.danger, "hover:bg-red-500/10 hover:text-red-700 dark:hover:text-red-400")}
                                onClick={() => cancel.mutate(j.id)}
                                disabled={cancel.isPending}
                            >
                                <Ban /> Cancel
                            </Button>
                        </div>
                    ) : (
                        <span className="block text-right text-subtle-foreground">—</span>
                    ),
            },
        ],
        [cancel],
    );

    const rows = data?.data ?? [];

    return (
        <>
            <div className="mb-1 flex justify-end">
                <StateLegend label="Job statuses explained" entries={GENERATION_JOB_LEGEND} />
            </div>
            <DataTable
                columns={columns}
                rows={rows}
                getRowId={(j) => j.id}
                loading={isLoading}
                error={error}
                onRetry={() => refetch()}
                onRowClick={(j) => setOpenId(j.id)}
                errorTitle="Failed to load jobs"
                storageKey="admin.warmup-content.jobs"
                csvName="warmbly-warmup-content-jobs"
                noun="jobs"
                emptyTitle="No generation jobs"
                emptyHint="Jobs appear here once the controller submits a generation run."
                pager={{
                    canPrev: pager.canPrev,
                    canNext: !!data?.pagination.has_more,
                    onPrev: pager.prev,
                    onNext: () => pager.next(data?.pagination.next_cursor),
                    page: pager.page,
                    shown: rows.length,
                    total: data?.pagination.total ?? null,
                }}
            />

            {openId && (
                <JobDetailDialog
                    id={openId}
                    open
                    onOpenChange={(v) => !v && setOpenId(null)}
                />
            )}
        </>
    );
}

function JobDetailDialog({
    id,
    open,
    onOpenChange,
}: {
    id: string;
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    // Poll while the job is still running so the drawer mirrors the live table.
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup-content", "job", id],
        queryFn: () => getWarmupGenerationJob(id),
        refetchInterval: (query) => {
            const j = query.state.data?.data;
            return j && isJobActive(j) ? 10_000 : false;
        },
    });
    const j = data?.data;
    const isBatch = j?.mode === "batch";

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        Generation job
                        {j ? (
                            <StatusBadge tone={jobTone(j.status)} dot>
                                {j.status}
                            </StatusBadge>
                        ) : null}
                    </DialogTitle>
                    <DialogDescription>
                        {j ? <span className="font-mono text-xs">{j.id}</span> : "Full job detail."}
                    </DialogDescription>
                </DialogHeader>

                {error ? (
                    <ErrorState error={error} title="Failed to load job" onRetry={() => refetch()} />
                ) : isLoading || !j ? (
                    <div className="space-y-2">
                        <Skeleton className="h-5 w-1/2" />
                        <Skeleton className="h-16" />
                        <Skeleton className="h-16" />
                    </div>
                ) : (
                    <div className="space-y-4">
                        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-4">
                            <Count label="Requested" value={j.requested_count} />
                            <Count label="Generated" value={j.generated_count} />
                            <Count
                                label="Lint rejected"
                                value={j.lint_rejected_count}
                                className={j.lint_rejected_count > 0 ? TONE_TEXT.warning : undefined}
                            />
                            <Count
                                label="Failed"
                                value={j.failed_count}
                                className={j.failed_count > 0 ? TONE_TEXT.danger : undefined}
                            />
                        </div>

                        {j.error && (
                            <Callout tone="danger" icon={CircleAlert} title="Error">
                                <span className="whitespace-pre-wrap">{j.error}</span>
                            </Callout>
                        )}

                        <div className="grid gap-x-8 sm:grid-cols-2">
                            <PropertyList>
                                <Property label="Mode">
                                    <ModeBadge mode={j.mode} />
                                </Property>
                                <Property label="Pool">
                                    <PoolBadge pool={j.pool_type} />
                                </Property>
                                <Property label="Segment">{j.segment || "—"}</Property>
                                <Property label="Trigger">{j.trigger || "—"}</Property>
                                <Property label="Model">{j.model || "—"}</Property>
                                <Property label="Theme">{j.theme || "—"}</Property>
                            </PropertyList>
                            <PropertyList>
                                {isBatch && (
                                    <>
                                        <Property label="Batch status">
                                            {j.batch_status ? (
                                                <StatusBadge tone={batchTone(j.batch_status)}>
                                                    {j.batch_status}
                                                </StatusBadge>
                                            ) : (
                                                "—"
                                            )}
                                        </Property>
                                        <Property label="Completion window">{j.completion_window || "—"}</Property>
                                        <Property label="Batch ID">
                                            <span className="font-mono text-xs break-all">{j.batch_id || "—"}</span>
                                        </Property>
                                    </>
                                )}
                                <Property label="Started">{fmtDate(j.started_at)}</Property>
                                <Property label="Finished">{fmtDate(j.finished_at)}</Property>
                                <Property label="Created">{fmtDate(j.created_at)}</Property>
                            </PropertyList>
                        </div>
                    </div>
                )}

                <DialogFooter showCloseButton />
            </DialogContent>
        </Dialog>
    );
}

function Count({ label, value, className }: { label: string; value: number; className?: string }) {
    return (
        <div className="bg-card px-3.5 py-2.5">
            <div className="text-xs text-muted-foreground">{label}</div>
            <div className={cn("mt-0.5 text-lg font-semibold tracking-[-0.01em] tabular-nums", className)}>
                {value.toLocaleString()}
            </div>
        </div>
    );
}
