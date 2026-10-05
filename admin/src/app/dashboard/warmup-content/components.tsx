// Small shared presentational components for the warmup-content section. Kept
// JSX-only (no constant/fn exports) so React Fast Refresh stays happy; pure
// helpers live in `shared.ts`.

import type { ReactNode } from "react";
import { StatusBadge } from "@/components/ui/kit";

export function PoolBadge({ pool }: { pool: string }) {
    return <StatusBadge tone={pool === "premium" ? "strong" : "neutral"}>{pool}</StatusBadge>;
}

export function ModeBadge({ mode }: { mode: string | null | undefined }) {
    return <StatusBadge tone={mode === "batch" ? "info" : "neutral"}>{mode ?? "sync"}</StatusBadge>;
}

// Header cell for the hand-rolled tables in this section.
export function Th({ children, right }: { children?: ReactNode; right?: boolean }) {
    return (
        <th
            className={`h-9 whitespace-nowrap px-3 text-xs font-medium text-muted-foreground first:pl-4 last:pr-4 ${
                right ? "text-right" : "text-left"
            }`}
        >
            {children}
        </th>
    );
}

export function Td({ children, right, className }: { children?: ReactNode; right?: boolean; className?: string }) {
    return (
        <td className={`px-3 first:pl-4 last:pr-4 ${right ? "text-right tabular-nums" : ""} ${className ?? ""}`}>
            {children}
        </td>
    );
}
