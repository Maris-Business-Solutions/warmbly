// Setup and health, findings: everything this deployment is currently
// getting wrong, as decided by the running backend. The endpoint returns
// only checks that are not ok, so an empty response is a real all-clear.

import { useState } from "react";
import { cn } from "@/lib/utils";
import { useSearchParams } from "react-router-dom";
import {
    AlertTriangle,
    ArrowUpCircle,
    CheckCircle2,
    Info,
    Loader2,
    RefreshCw,
    XCircle,
} from "lucide-react";
import { ErrorState } from "@/components/ErrorState";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Callout, Stat, StatGrid } from "@/components/ui/kit";
import { TONE_PANEL, TONE_TEXT } from "@/lib/tones";
import { InstanceFindings } from "../InstanceHealthPanel";
import { UpdateDialog } from "@/components/layout/UpdateDialog";
import { useInstanceHealth } from "@/hooks/useInstanceHealth";
import { buildLabel, isUpdating, useUpdateState } from "@/hooks/useUpdateState";
import type { InstanceHealthSummary } from "@/lib/api/client/admin/instance";

export function FindingsTab() {
    const healthQ = useInstanceHealth();

    const checks = healthQ.data?.checks ?? [];
    const summary = healthQ.data?.summary;

    return (
        <div>
            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
                <p className="max-w-2xl text-[12.5px] leading-relaxed text-muted-foreground">
                    Checks the backend runs against this instance: secrets, addresses, platform
                    mail, accounts, workers and storage. Only findings that need a decision are
                    listed.
                </p>
                <div className="flex items-center gap-2">
                    {healthQ.dataUpdatedAt > 0 && (
                        <span className="text-xs text-subtle-foreground tabular-nums">
                            Last checked {new Date(healthQ.dataUpdatedAt).toLocaleTimeString()}
                        </span>
                    )}
                    <Button
                        size="sm"
                        variant="outline"
                        onClick={() => healthQ.refetch()}
                        disabled={healthQ.isFetching}
                    >
                        <RefreshCw className={cn("size-3.5", healthQ.isFetching && "animate-spin")} />
                        {healthQ.isFetching ? "Checking..." : "Run checks"}
                    </Button>
                </div>
            </div>

            <UpdateCard />

            {healthQ.isLoading && (
                <div className="space-y-3">
                    <Skeleton className="h-[86px] w-full" />
                    <Skeleton className="h-20 w-full" />
                    <Skeleton className="h-20 w-full" />
                </div>
            )}

            {healthQ.isError && (
                <ErrorState
                    error={healthQ.error}
                    title="Could not run the instance checks"
                    onRetry={() => healthQ.refetch()}
                />
            )}

            {healthQ.data && (
                <>
                    {checks.length === 0 ? (
                        <Callout tone="success" icon={CheckCircle2} title="No problems found">
                            Every setup and health check passed. This tab lists only the checks that
                            need attention, so it stays empty while the instance is configured
                            correctly.
                        </Callout>
                    ) : (
                        <>
                            <SummaryStrip summary={summary} total={checks.length} />
                            <InstanceFindings checks={checks} />
                        </>
                    )}
                </>
            )}
        </div>
    );
}

// Version and update status, above the findings: the same facts as the pill
// in the top bar, on the page an operator opens to ask "is this instance ok".
function UpdateCard() {
    const updateQ = useUpdateState();
    // ?update=1 is how the dashboard's version pill deep-links an admin
    // straight into the dialog.
    const [params] = useSearchParams();
    const [open, setOpen] = useState(params.get("update") === "1");
    const state = updateQ.data;
    if (!state) return null;

    const updating = isUpdating(state);
    const available = state.update_available;
    const panel = updating ? TONE_PANEL.info : available ? TONE_PANEL.warning : "border-border bg-card";

    return (
        <div className={cn("mb-6 flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3", panel)}>
            {updating ? (
                <Loader2 className={cn("size-4 shrink-0 animate-spin", TONE_TEXT.info)} />
            ) : available ? (
                <ArrowUpCircle className={cn("size-4 shrink-0", TONE_TEXT.warning)} />
            ) : (
                <CheckCircle2 className={cn("size-4 shrink-0", TONE_TEXT.success)} />
            )}
            <div className="min-w-0 flex-1 text-[13px]">
                <span className="font-medium text-foreground">
                    {updating
                        ? "Updating this instance"
                        : available
                          ? `${state.latest?.tag && state.reason === "release" ? state.latest.tag : "A newer version"} is available`
                          : "Up to date"}
                </span>
                <span className="text-muted-foreground">
                    {" "}
                    running {buildLabel(state)}
                    {state.updater.checkout && !state.updater.checkout.detached
                        ? ` on ${state.updater.checkout.branch}`
                        : ""}
                    {state.checked_at
                        ? `, checked ${new Date(state.checked_at).toLocaleTimeString()}`
                        : ""}
                </span>
            </div>
            <Button size="sm" variant={available ? "default" : "outline"} onClick={() => setOpen(true)}>
                {updating ? "Progress" : available ? "Update" : "Details"}
            </Button>
            <UpdateDialog open={open} onOpenChange={setOpen} />
        </div>
    );
}

function SummaryStrip({
    summary,
    total,
}: {
    summary: InstanceHealthSummary | undefined;
    total: number;
}) {
    const errors = summary?.error ?? 0;
    const warnings = summary?.warning ?? 0;
    const info = summary?.info ?? 0;

    return (
        <StatGrid className="mb-6 grid-cols-1 sm:grid-cols-3 md:grid-cols-3">
            <Stat
                icon={XCircle}
                label="Errors"
                value={errors}
                sub="Something is broken right now"
                tone={errors > 0 ? "danger" : undefined}
            />
            <Stat
                icon={AlertTriangle}
                label="Warnings"
                value={warnings}
                sub="Works, but not the way you want"
                tone={warnings > 0 ? "warning" : undefined}
            />
            <Stat
                icon={Info}
                label="Worth knowing"
                value={info}
                sub={`${total} findings in total`}
                tone={info > 0 ? "info" : undefined}
            />
        </StatGrid>
    );
}
