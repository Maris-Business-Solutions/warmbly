// Workspace abuse posture — the operator view. The customer endpoint returns
// the band and a sentence; this is where the evidence behind it is readable,
// and the only place a posture can be changed or lifted.

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Pin, PinOff, ShieldAlert, ShieldCheck, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Callout, EmptyState, StatusBadge } from "@/components/ui/kit";
import { ErrorState } from "@/components/ErrorState";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import { TONE_TEXT, type Tone } from "@/lib/tones";
import {
    clearOrganizationRiskOverride,
    clearOrganizationRiskSignal,
    getOrganizationRisk,
} from "@/lib/api/client/admin/organizations";
import type { OrgRisk, OrgRiskSignal, OrgRiskState } from "@/lib/api/models/admin";
import { OrganizationRiskDialog } from "./OrganizationRiskDialog";

/** Band tones, in one place so the pill reads the same everywhere. */
const BAND_TONE: Record<OrgRiskState, Tone> = {
    trusted: "success",
    watch: "warning",
    restricted: "orange",
    suspended: "danger",
};

const BAND_EFFECT: Record<OrgRiskState, string> = {
    trusted: "Nothing is restricted.",
    watch: "Nothing the workspace can feel. Evidence is accumulating.",
    restricted: "Daily volume per mailbox is a quarter, and warmup is on the free pool.",
    suspended: "Sending is stopped entirely.",
};

export function RiskBadge({ state }: { state: OrgRiskState }) {
    return (
        <StatusBadge tone={BAND_TONE[state]} dot>
            {state}
        </StatusBadge>
    );
}

export function OrganizationRiskCard({ orgId }: { orgId: string }) {
    const qc = useQueryClient();
    const [editing, setEditing] = useState(false);
    // Reading the evidence needs only view_organizations; every write below is
    // gated on manage_organizations, so an admin without it is shown the
    // record rather than buttons that can only answer 403.
    const canManage = useAdminPerm(AdminPerm.ManageOrganizations);

    const riskQuery = useQuery({
        queryKey: ["admin", "organizations", orgId, "risk"],
        queryFn: () => getOrganizationRisk(orgId),
        enabled: !!orgId,
    });

    // Both writes return the whole record, so seed the cache from the response
    // rather than refetching, and refresh the org row whose band is inlined.
    function applied(message: string) {
        return (risk: OrgRisk) => {
            qc.setQueryData(["admin", "organizations", orgId, "risk"], risk);
            qc.invalidateQueries({ queryKey: ["admin", "organizations", orgId] });
            qc.invalidateQueries({ queryKey: ["admin", "organizations"] });
            toast.success(message);
        };
    }

    const liftMutation = useMutation({
        mutationFn: () => clearOrganizationRiskOverride(orgId),
        onSuccess: applied("Override lifted; the evidence decides again"),
        onError: (e: Error) => toast.error(e.message || "Failed to lift the override"),
    });

    const retractMutation = useMutation({
        mutationFn: (key: string) => clearOrganizationRiskSignal(orgId, key),
        onSuccess: applied("Finding retracted"),
        onError: (e: Error) => toast.error(e.message || "Failed to retract the finding"),
    });

    if (riskQuery.isLoading) return <Skeleton className="h-40 w-full rounded-lg" />;
    if (riskQuery.error || !riskQuery.data) {
        return (
            <ErrorState
                error={riskQuery.error}
                title="Failed to load the workspace posture."
                onRetry={() => riskQuery.refetch()}
            />
        );
    }

    const risk = riskQuery.data;
    const signals = Object.entries(risk.signals ?? {});
    const pinned = risk.override ?? null;

    return (
        <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
            <div className="flex flex-wrap items-start justify-between gap-3 px-4 py-3">
                <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                        <RiskBadge state={risk.state} />
                        <span className="text-[13px] font-medium tabular-nums text-foreground">
                            Score {risk.score}
                        </span>
                        {pinned && (
                            <span className="inline-flex items-center gap-1 text-xs text-[var(--admin-accent-strong)]">
                                <Pin className="size-3" /> pinned by an operator
                            </span>
                        )}
                    </div>
                    <p className="mt-1.5 text-[12.5px] text-muted-foreground">{BAND_EFFECT[risk.state]}</p>
                    {risk.reason && (
                        <p className="mt-1 text-[12.5px] text-foreground">
                            <span className="text-muted-foreground">Shown to the workspace: </span>
                            {risk.reason}
                        </p>
                    )}
                    {risk.evaluated_at && (
                        <p className="mt-1 text-xs text-subtle-foreground">
                            Last evaluated {new Date(risk.evaluated_at).toLocaleString()}
                        </p>
                    )}
                </div>
                {canManage && (
                    <div className="flex shrink-0 items-center gap-1.5">
                        {pinned && (
                            <Button
                                size="sm"
                                variant="outline"
                                onClick={() => liftMutation.mutate()}
                                disabled={liftMutation.isPending}
                            >
                                <PinOff className="size-3.5" />
                                {liftMutation.isPending ? "Lifting…" : "Lift override"}
                            </Button>
                        )}
                        <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
                            <ShieldAlert className="size-3.5" />
                            Set posture
                        </Button>
                    </div>
                )}
            </div>

            {pinned && (
                <div className="px-4 pb-3">
                    <Callout tone="accent" icon={Pin}>
                        <span className="text-foreground">
                            Pinned to <span className="font-medium">{pinned.state}</span>
                            {pinned.at && <> on {new Date(pinned.at).toLocaleString()}</>}
                            {pinned.reason && <>: "{pinned.reason}"</>}
                        </span>
                        <span className="mt-0.5 block text-xs">
                            Detectors keep scoring the evidence, but the band stays here until the override is lifted.
                        </span>
                    </Callout>
                </div>
            )}

            <div className="border-t border-border">
                <SignalsTable
                    signals={signals}
                    onRetract={canManage ? (key) => retractMutation.mutate(key) : undefined}
                    busyKey={retractMutation.isPending ? retractMutation.variables : undefined}
                />
            </div>

            {canManage && (
                <OrganizationRiskDialog
                    orgId={orgId}
                    risk={risk}
                    open={editing}
                    onOpenChange={setEditing}
                />
            )}
        </div>
    );
}

function SignalsTable({
    signals,
    onRetract,
    busyKey,
}: {
    signals: [string, OrgRiskSignal][];
    // Absent when the admin can read the evidence but not change it.
    onRetract?: (key: string) => void;
    busyKey?: string;
}) {
    if (signals.length === 0) {
        return (
            <EmptyState
                icon={ShieldCheck}
                title="No findings"
                hint="No detector has filed anything against this workspace."
                className="py-8"
            />
        );
    }
    // Heaviest first, matching the sentence the customer is shown.
    const rows = [...signals].sort(
        (a, b) => (b[1].weight ?? 0) - (a[1].weight ?? 0),
    );
    return (
        <div className="overflow-x-auto">
            <table className="w-full border-collapse text-[13px]">
                <thead>
                    <tr className="border-b border-border">
                        <th className="h-9 px-3 pl-4 text-left text-xs font-medium text-muted-foreground">Detector</th>
                        <th className="h-9 px-3 text-right text-xs font-medium text-muted-foreground">Weight</th>
                        <th className="h-9 px-3 text-left text-xs font-medium text-muted-foreground">Finding</th>
                        <th className="h-9 px-3 pr-4 text-left text-xs font-medium text-muted-foreground">Ages out</th>
                        {onRetract && <th className="h-9 w-10 pr-4" />}
                    </tr>
                </thead>
                <tbody>
                    {rows.map(([key, signal]) => {
                        const expires = signal.expires_at
                            ? new Date(signal.expires_at)
                            : null;
                        const expired = expires ? expires.getTime() < Date.now() : false;
                        return (
                            <tr
                                key={key}
                                className="group border-b border-border/70 transition-colors last:border-0 hover:bg-accent/50"
                            >
                                <td className="h-10 px-3 pl-4 font-mono text-[12px] font-medium text-foreground">{key}</td>
                                <td className="px-3 text-right tabular-nums">
                                    {signal.weight ?? 0}
                                </td>
                                <td className="px-3 py-2 text-muted-foreground">
                                    {signal.detail || "—"}
                                </td>
                                <td className="whitespace-nowrap px-3 pr-4 text-xs text-muted-foreground">
                                    {expires ? (
                                        expired ? (
                                            <span className={TONE_TEXT.warning}>
                                                expired, awaiting sweep
                                            </span>
                                        ) : (
                                            expires.toLocaleDateString()
                                        )
                                    ) : (
                                        "only when retracted"
                                    )}
                                </td>
                                {onRetract && (
                                    <td className="pr-4 text-right">
                                        <Button
                                            type="button"
                                            size="icon-xs"
                                            variant="ghost"
                                            title="Retract this finding"
                                            aria-label="Retract this finding"
                                            onClick={() => onRetract(key)}
                                            disabled={busyKey === key}
                                            className="opacity-100 hover:text-red-600 focus-visible:opacity-100 md:opacity-0 md:group-hover:opacity-100 dark:hover:text-red-400"
                                        >
                                            <X className="size-3.5" />
                                        </Button>
                                    </td>
                                )}
                            </tr>
                        );
                    })}
                </tbody>
            </table>
        </div>
    );
}
