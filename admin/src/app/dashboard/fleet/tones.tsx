// Status pills shared by the fleet pages, on the one palette in lib/tones so
// they read the same as WORKER_HEALTH_LEGEND and everywhere else.

import { StatusBadge, StatusDot } from "@/components/ui/kit";
import type { Tone } from "@/lib/tones";
import type { NodeState } from "@/lib/api/client/admin/fleetNodes";

const HEALTH_TONE: Record<string, Tone> = {
    healthy: "success",
    watch: "warning",
    throttled: "orange",
    quarantined: "danger",
    blocked: "danger",
};

const NODE_STATE_TONE: Record<NodeState, Tone> = {
    live: "success",
    unreachable: "warning",
    stopped: "neutral",
};

export function HealthPill({ state }: { state: string }) {
    return (
        <StatusBadge tone={HEALTH_TONE[state] ?? "neutral"} dot>
            {state || "unknown"}
        </StatusBadge>
    );
}

export function NodeStatePill({ state }: { state: NodeState }) {
    return (
        <StatusBadge tone={NODE_STATE_TONE[state]} dot>
            {state}
        </StatusBadge>
    );
}

export function LiveDot({ live, title }: { live: boolean; title?: string }) {
    return (
        <span title={title}>
            <StatusDot tone={live ? "success" : "neutral"} className="text-[12.5px]">
                <span className={live ? "text-foreground" : "text-muted-foreground"}>{live ? "live" : "offline"}</span>
            </StatusDot>
        </span>
    );
}
