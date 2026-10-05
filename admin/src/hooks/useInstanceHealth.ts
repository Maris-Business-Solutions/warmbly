import { useQuery } from "@tanstack/react-query";
import {
    getInstanceHealth,
    type CheckSeverity,
    type InstanceHealthResult,
} from "@/lib/api/client/admin/instance";

export const INSTANCE_HEALTH_KEY = ["admin", "instance", "health"] as const;

// One cache entry feeds the sidebar badge, the Overview panel and the
// Setup and health page, so they can never disagree.
export function useInstanceHealth(options?: { enabled?: boolean }) {
    return useQuery({
        queryKey: INSTANCE_HEALTH_KEY,
        queryFn: getInstanceHealth,
        refetchInterval: 60_000,
        retry: false,
        enabled: options?.enabled ?? true,
    });
}

// Informational notes are not problems and must not inflate the nav badge.
export function findingCount(data: InstanceHealthResult | undefined): number {
    return data?.checks?.filter((c) => c.severity === "error" || c.severity === "warning").length ?? 0;
}

export function worstSeverity(
    data: InstanceHealthResult | undefined,
): CheckSeverity | null {
    const checks = data?.checks ?? [];
    if (checks.some((c) => c.severity === "error")) return "error";
    if (checks.some((c) => c.severity === "warning")) return "warning";
    return null;
}
