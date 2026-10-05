// Who has actually used a code.
//
// The value columns are the snapshot taken at redemption, not the code's
// current value, so a code edited after the fact still shows what each
// workspace was really given.

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Callout, EmptyState, StatusBadge } from "@/components/ui/kit";
import { Skeleton } from "@/components/ui/skeleton";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { useCursorPager } from "@/lib/useCursorPager";
import type { Tone } from "@/lib/tones";
import { listDiscountRedemptions } from "@/lib/api/client/admin/discounts";
import type { DiscountCode, DiscountRedemption, DiscountRedemptionStatus } from "@/lib/api/models/admin";
import { AlertTriangle, Receipt } from "lucide-react";
import { describeDiscount, formatMoney } from "./summary";

const STATUS_TONE: Record<DiscountRedemptionStatus, Tone> = {
    applied: "success",
    pending: "warning",
    canceled: "neutral",
};

function grantedValue(r: DiscountRedemption): string {
    if (r.percent_off != null) return `${r.percent_off}% off`;
    if (r.amount_off != null) return `${formatMoney(r.amount_off, r.currency)} off`;
    if (r.trial_extension_days != null) return `${r.trial_extension_days} trial days`;
    return "—";
}

export function DiscountRedemptionsDialog({
    discount,
    open,
    onOpenChange,
}: {
    discount: DiscountCode;
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const pager = useCursorPager();

    const { data, isLoading, isError, isFetching, isPlaceholderData } = useQuery({
        queryKey: ["admin", "discounts", discount.id, "redemptions", pager.cursor],
        queryFn: () => listDiscountRedemptions(discount.id, { cursor: pager.cursor, limit: 50 }),
        enabled: open,
        placeholderData: keepPreviousData,
    });

    const rows = data?.data ?? [];

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="flex max-h-[85vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
                <DialogHeader className="shrink-0 px-5 pt-5 pb-4">
                    <DialogTitle className="flex items-center gap-2">
                        <span className="font-mono">{discount.code}</span>
                        <span className="font-normal text-muted-foreground">redemptions</span>
                    </DialogTitle>
                    <DialogDescription className="tabular-nums">
                        {describeDiscount(discount)} · redeemed{" "}
                        {discount.times_redeemed.toLocaleString()}
                        {discount.max_redemptions != null
                            ? ` of ${discount.max_redemptions.toLocaleString()}`
                            : ""}{" "}
                        time{discount.times_redeemed === 1 ? "" : "s"}
                    </DialogDescription>
                </DialogHeader>

                <div className="min-h-0 flex-1 overflow-y-auto border-t border-border">
                    {isLoading ? (
                        <div className="space-y-2 p-4">
                            <Skeleton className="h-8 w-full" />
                            <Skeleton className="h-8 w-full" />
                            <Skeleton className="h-8 w-full" />
                        </div>
                    ) : isError ? (
                        <div className="p-4">
                            <Callout tone="danger" icon={AlertTriangle}>
                                Could not load redemptions, so this list is not authoritative.
                            </Callout>
                        </div>
                    ) : rows.length === 0 ? (
                        <EmptyState icon={Receipt} title="Nobody has redeemed this code yet." className="py-10" />
                    ) : (
                        <div className="overflow-x-auto">
                            <table className="w-full min-w-[520px] text-[13px]">
                                <thead className="sticky top-0 z-10 bg-background">
                                    <tr className="h-9 border-b border-border text-xs text-muted-foreground">
                                        <th className="pr-3 pl-5 text-left font-medium">Workspace</th>
                                        <th className="px-3 text-left font-medium">Granted</th>
                                        <th className="px-3 text-left font-medium">Status</th>
                                        <th className="pr-5 pl-3 text-left font-medium">Redeemed</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {rows.map((r) => (
                                        <tr key={r.id} className="h-10 border-b border-border/70 last:border-b-0 hover:bg-accent/50">
                                            <td className="pr-3 pl-5">
                                                <Link
                                                    to={`/organizations/${r.organization_id}`}
                                                    className="font-mono text-xs text-[var(--admin-accent-strong)] hover:underline"
                                                >
                                                    {r.organization_id.slice(0, 8)}
                                                </Link>
                                            </td>
                                            <td className="px-3 tabular-nums">{grantedValue(r)}</td>
                                            <td className="px-3">
                                                <StatusBadge tone={STATUS_TONE[r.status]} dot className="capitalize">
                                                    {r.status}
                                                </StatusBadge>
                                            </td>
                                            <td className="pr-5 pl-3 text-xs whitespace-nowrap text-muted-foreground tabular-nums">
                                                {new Date(r.redeemed_at).toLocaleString()}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                </div>

                {(pager.canPrev || data?.pagination?.has_more) && (
                    <div className="flex shrink-0 items-center justify-end gap-1.5 border-t border-border px-5 py-2.5">
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={!pager.canPrev}
                            onClick={pager.prev}
                        >
                            Previous
                        </Button>
                        <Button
                            size="sm"
                            variant="outline"
                            // keepPreviousData keeps the old page's cursor on
                            // screen while the next one loads, and the pager
                            // appends without deduplicating, so a second click
                            // would push the same cursor twice.
                            disabled={!data?.pagination?.has_more || isFetching || isPlaceholderData}
                            onClick={() => pager.next(data?.pagination?.next_cursor)}
                        >
                            Next
                        </Button>
                    </div>
                )}
            </DialogContent>
        </Dialog>
    );
}
