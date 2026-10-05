// Status and kind pills for transfer jobs, shared by the instance-wide list
// and the per-workspace tab.

import { Loader2 } from "lucide-react";
import { StatusBadge } from "@/components/ui/kit";
import type { OrgTransferStatus } from "@/lib/api/client/admin/transfers";
import type { Tone } from "@/lib/tones";

const STATUS_TONE: Record<OrgTransferStatus, Tone> = {
    queued: "warning",
    running: "info",
    completed: "success",
    failed: "danger",
    expired: "neutral",
};

export function StatusPill({
    status,
    progress,
    stage,
}: {
    status: OrgTransferStatus;
    progress?: number;
    stage?: string;
}) {
    const running = status === "running";
    return (
        <div className="flex flex-col gap-0.5">
            <StatusBadge tone={STATUS_TONE[status] ?? "neutral"} dot={!running}>
                {running && <Loader2 className="size-3 animate-spin" />}
                {status}
                {running && progress != null && <span className="tabular-nums">{progress}%</span>}
            </StatusBadge>
            {running && stage && <span className="text-[11px] text-muted-foreground">{stage}</span>}
        </div>
    );
}

export function KindPill({ kind }: { kind: "export" | "import" }) {
    return <StatusBadge tone={kind === "export" ? "neutral" : "strong"}>{kind}</StatusBadge>;
}
