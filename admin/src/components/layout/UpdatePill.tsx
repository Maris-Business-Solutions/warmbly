// The version pill at the foot of the sidebar. Quiet when the instance is current, amber
// when a newer version exists, a spinner while an update runs or the backend
// restarts. It also closes the loop after a restart: once the backend answers
// with a new build it says "Updated to vX" and refreshes every query.

import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowUpCircle, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import { buildLabel, isUpdating, useUpdateState } from "@/hooks/useUpdateState";
import { clearUpdateStarted, readUpdateStarted } from "@/lib/updateSession";
import { UpdateDialog } from "./UpdateDialog";

export function UpdatePill() {
    const canRead = useAdminPerm(AdminPerm.ViewAnalytics);
    const stateQ = useUpdateState({ enabled: canRead });
    const qc = useQueryClient();
    const [open, setOpen] = useState(false);
    const state = stateQ.data;
    const updating = isUpdating(state);
    const started = readUpdateStarted();
    // The poll fails while the backend restarts; an update this browser
    // started is the only reason to read that as "restarting" rather than down.
    const restarting = !!started && stateQ.isError;

    // The backend is back after an update this browser started: report the
    // outcome once, then refresh everything so no page shows stale data.
    useEffect(() => {
        if (!started || !state || updating) return;
        const last = state.updater.last_job;
        const moved =
            (state.running.commit && state.running.commit !== started.fromCommit) ||
            (state.running.version && state.running.version !== started.fromVersion);
        if (last?.status === "failed") {
            clearUpdateStarted();
            toast.error(`The update failed: ${last.error ?? "see the update dialog"}`);
            return;
        }
        if (moved || last?.status === "succeeded") {
            clearUpdateStarted();
            toast.success(`Updated to ${buildLabel(state)}`);
            void qc.invalidateQueries();
        }
    }, [started, state, updating, qc]);

    if (!canRead || (!state && !restarting)) return null;

    let tone = "text-subtle-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground";
    let label = buildLabel(state);
    let icon: React.ReactNode = null;
    let title = "Up to date";

    if (restarting) {
        tone = "bg-sky-500/10 text-sky-700 dark:text-sky-400";
        label = "Restarting";
        icon = <Loader2 className="size-3 animate-spin" />;
        title = "The backend is restarting after an update";
    } else if (updating) {
        tone = "bg-sky-500/10 text-sky-700 dark:text-sky-400";
        label = "Updating";
        icon = <Loader2 className="size-3 animate-spin" />;
        title = `Update in progress: ${state?.updater.job?.step ?? ""}`;
    } else if (state?.update_available) {
        tone = "bg-[var(--admin-accent-soft)] text-[var(--admin-accent-strong)] hover:bg-[color-mix(in_oklab,var(--admin-accent)_22%,transparent)]";
        label = state.latest?.tag && state.reason === "release" ? `Update to ${state.latest.tag}` : "Update available";
        icon = (
            <span className="relative flex size-3.5 items-center justify-center">
                <span className="absolute inline-flex size-full rounded-full bg-[var(--admin-accent)] opacity-40 animate-ping" />
                <ArrowUpCircle className="relative size-3.5" />
            </span>
        );
        title = `A newer version is available; running ${buildLabel(state)}`;
    } else if (state?.updater.status === "unreachable") {
        tone = "text-amber-700 hover:bg-sidebar-accent dark:text-amber-400";
        title = "The updater is not answering";
    }

    return (
        <>
            <button
                type="button"
                onClick={() => setOpen(true)}
                title={title}
                className={cn(
                    "inline-flex h-6 items-center gap-1.5 rounded-md px-1.5 text-[12px] font-medium tabular-nums transition-colors",
                    tone,
                )}
            >
                {icon}
                {label}
            </button>
            <UpdateDialog open={open} onOpenChange={setOpen} />
        </>
    );
}
