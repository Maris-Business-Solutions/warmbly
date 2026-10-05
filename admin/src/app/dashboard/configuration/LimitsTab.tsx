// Configuration, limits: what this instance is actually enforcing right now,
// after configuration, plan and product defaults have all been applied.
// Read only, because every number here is owned by one of those layers.

import { useQuery } from "@tanstack/react-query";
import { Gauge, SlidersHorizontal } from "lucide-react";
import { ErrorState } from "@/components/ErrorState";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/kit";
import { SettingsGroup, SettingsRow } from "./SettingsLayout";
import {
    getInstanceLimits,
    type InstanceLimitGroup,
} from "@/lib/api/client/admin/instance";

interface LimitsTabProps {
    onSwitchTab?: (tab: "environment") => void;
}

export function LimitsTab({ onSwitchTab }: LimitsTabProps) {
    const limitsQ = useQuery({
        queryKey: ["admin", "instance", "limits"],
        queryFn: getInstanceLimits,
        retry: false,
    });

    const groups = limitsQ.data?.groups ?? [];

    return (
        <div>
            <div className="mb-8 flex flex-wrap items-start justify-between gap-3">
                <p className="max-w-2xl text-[13px] leading-relaxed text-muted-foreground">
                    The caps and defaults this instance enforces, resolved from configuration and
                    the product defaults. Read only: change the matching variable or the
                    organization&apos;s override instead.
                </p>
                <Button size="sm" variant="outline" onClick={() => onSwitchTab?.("environment")}>
                    <SlidersHorizontal />
                    Environment
                </Button>
            </div>

            {limitsQ.isLoading && (
                <div className="space-y-10">
                    {[0, 1].map((i) => (
                        <div key={i}>
                            <Skeleton className="mb-3 h-4 w-32" />
                            <Skeleton className="h-40 w-full rounded-lg" />
                        </div>
                    ))}
                </div>
            )}

            {limitsQ.isError && (
                <ErrorState
                    error={limitsQ.error}
                    title="Could not load the effective limits"
                    onRetry={() => limitsQ.refetch()}
                />
            )}

            {limitsQ.data && groups.length === 0 && (
                <div className="rounded-lg border border-dashed border-border">
                    <EmptyState icon={Gauge} title="The backend returned no limits." />
                </div>
            )}

            {groups.map((group) => (
                <LimitGroup key={group.title} group={group} />
            ))}
        </div>
    );
}

function LimitGroup({ group }: { group: InstanceLimitGroup }) {
    const entries = group.entries ?? [];
    return (
        <SettingsGroup
            title={group.title}
            actions={
                <span className="text-xs text-muted-foreground tabular-nums">{entries.length}</span>
            }
        >
            {entries.length === 0 && (
                <div className="px-4 py-3 text-[12.5px] text-muted-foreground">No limits in this group.</div>
            )}
            {entries.map((entry) => (
                <SettingsRow key={entry.name} label={entry.name} description={entry.description}>
                    <div className="text-right">
                        <div className="text-[13px] font-medium text-foreground tabular-nums">{entry.value}</div>
                        {entry.unit && <div className="text-xs text-muted-foreground">{entry.unit}</div>}
                    </div>
                </SettingsRow>
            ))}
        </SettingsGroup>
    );
}
