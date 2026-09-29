// Small presentational pieces shared by the batch list and the batch page.
import { DitherStack } from "@/components/ui/dither";
import { cn } from "@/lib/utils";
import type {
    PlacementBatchProgress,
    PlacementBatchSenderStatus,
    PlacementBatchStatus,
} from "@/lib/api/models/app/placement/Placement";
import { BATCH_STATUS, SENDER_STATUS, doneCount, progressLine, progressSegments } from "./placementBatches";

export function BatchStatusChip({ status, progress }: { status: PlacementBatchStatus; progress?: PlacementBatchProgress }) {
    const s = BATCH_STATUS[status] ?? BATCH_STATUS.failed;
    const counter = status === "running" && progress ? ` ${doneCount(progress).toLocaleString()}/${progress.total.toLocaleString()}` : "";
    return (
        <span className={cn("inline-flex items-center gap-1.5 h-5 px-1.5 rounded-md border text-[10.5px] font-medium whitespace-nowrap", s.chip)}>
            <span className={cn("size-1.5 rounded-full", s.dot)} />
            {s.label}
            {counter && <span className="font-mono tabular-nums">{counter}</span>}
        </span>
    );
}

export function SenderStatusChip({ status }: { status: PlacementBatchSenderStatus }) {
    const s = SENDER_STATUS[status] ?? SENDER_STATUS.failed;
    return (
        <span className={cn("inline-flex items-center gap-1.5 h-5 px-1.5 rounded-md border text-[10.5px] font-medium whitespace-nowrap", s.chip)}>
            <span className={cn("size-1.5 rounded-full", s.dot)} />
            {s.label}
        </span>
    );
}

// Senders done or sending as shares of the whole batch; the rest is the empty track.
export function BatchProgressBar({ progress, height = 6, className }: { progress: PlacementBatchProgress; height?: number; className?: string }) {
    return (
        <div title={progressLine(progress)} className={className}>
            <DitherStack segments={progressSegments(progress)} height={height} />
        </div>
    );
}
