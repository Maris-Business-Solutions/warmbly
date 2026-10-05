// Warmup appeals review surface. Where the Warmup page is the at-a-glance
// pool monitor, this page is the focused enforcement queue: admins triage
// warmup-ban appeals (approve = unblock the mailbox, reject = stays blocked)
// and can unblock blocked mailboxes directly.
//
//   - Appeals tab: a triage view. The list is filterable by status
//     (pending/approved/rejected); the selected appeal opens in a detail pane
//     with its reason, history and the approve/reject decision, which takes
//     optional review notes.
//   - Blocked mailboxes tab: every mailbox currently blocked from the pool,
//     with whether it has an open appeal and a direct (confirmed) unblock.
//
// Mirrors LimitRequestsPage's review pattern so the two enforcement queues
// read the same way.

import { useEffect, useRef, useState } from "react";
import {
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";
import { AlertTriangle, CheckCircle2, Inbox, ShieldCheck, ShieldOff, XCircle } from "lucide-react";
import { useConfirm } from "@/components/ConfirmDialog";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageTabs } from "@/components/layout/PageTabs";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
    Callout,
    EmptyState,
    Property,
    PropertyList,
    Segmented,
    StatusBadge,
} from "@/components/ui/kit";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useCursorPager } from "@/lib/useCursorPager";
import {
    approveAppeal,
    listBlockedWarmupAccounts,
    listWarmupAppeals,
    rejectAppeal,
    unblockWarmupAccount,
} from "@/lib/api/client/admin/warmup";
import type {
    AdminBlockedAccount,
    WarmupAppeal,
    WarmupAppealStatus,
} from "@/lib/api/models/admin";
import { TONE_TEXT, type Tone } from "@/lib/tones";
import { cn } from "@/lib/utils";

const STATUS_TONE: Record<WarmupAppealStatus, Tone> = {
    pending: "warning",
    approved: "success",
    rejected: "danger",
};

type AppealStatusFilter = WarmupAppealStatus | "all";
type Mode = "approve" | "reject";

const STATUS_OPTIONS: { value: AppealStatusFilter; label: string }[] = [
    { value: "pending", label: "Pending" },
    { value: "approved", label: "Approved" },
    { value: "rejected", label: "Rejected" },
    { value: "all", label: "All" },
];

const TABS = [
    { id: "appeals", label: "Appeals", icon: Inbox },
    { id: "blocked", label: "Blocked mailboxes", icon: ShieldOff },
];

// The detail pane sits beside the list from this width; below it, it stacks.
const SIDE_BY_SIDE = "(min-width: 1280px)";

const cap = (s: string) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : s);

function mailboxOf(a: WarmupAppeal): string {
    return a.email_account?.email ?? a.email_account_id;
}

export default function WarmupAppealsPage() {
    const [tab, setTab] = useState("appeals");

    return (
        <div>
            <PageHeader
                title="Warmup appeals"
                description="Review warmup-ban appeals and unblock mailboxes. Approving an appeal unblocks the mailbox and re-admits it to the pool; rejecting keeps it blocked. Shared paid-pool reputation matters more than any single mailbox."
            />

            <PageTabs tabs={TABS} value={tab} onChange={setTab} />

            {tab === "appeals" ? <AppealsTab /> : <BlockedTab />}
        </div>
    );
}

// One mutation for both decisions, shared by the detail pane and the dialog.
function useReviewAppeal(appeal: WarmupAppeal, onDone: () => void) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: ({ mode, notes }: { mode: Mode; notes: string }) =>
            mode === "approve"
                ? approveAppeal(appeal.id, { approved: true, notes })
                : rejectAppeal(appeal.id, { approved: false, notes }),
        onSuccess: (_res, { mode }) => {
            toast.success(`Appeal ${mode === "approve" ? "approved" : "rejected"}`);
            // Approving unblocks the mailbox, so refresh both queues.
            qc.invalidateQueries({ queryKey: ["admin", "warmup"] });
            onDone();
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });
}

function AppealsTab() {
    const [status, setStatus] = useState<AppealStatusFilter>("pending");
    const pager = useCursorPager();
    const { reset } = pager;
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const detailRef = useRef<HTMLDivElement>(null);

    const [reviewing, setReviewing] = useState<{
        appeal: WarmupAppeal;
        mode: Mode;
    } | null>(null);

    useEffect(() => {
        reset();
    }, [status, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "appeals", status, pager.cursor],
        queryFn: () => listWarmupAppeals(status, pager.cursor),
        placeholderData: keepPreviousData,
        staleTime: 10_000,
    });

    const rows = data?.data ?? [];
    // A decided appeal leaves the pending list; selection then falls to the next one.
    const selected = rows.find((r) => r.id === selectedId) ?? rows[0] ?? null;

    function select(a: WarmupAppeal) {
        setSelectedId(a.id);
        if (!window.matchMedia(SIDE_BY_SIDE).matches) {
            detailRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
        }
    }

    const columns: Column<WarmupAppeal>[] = [
        {
            id: "mailbox",
            header: "Mailbox",
            cell: (a) => <span className="block truncate font-medium text-foreground">{mailboxOf(a)}</span>,
            csv: (a) => mailboxOf(a),
        },
        {
            id: "user",
            header: "User",
            cell: (a) => <span className="text-muted-foreground">{a.user?.email ?? a.user_id}</span>,
            csv: (a) => a.user?.email ?? a.user_id,
        },
        {
            id: "reason",
            header: "Reason",
            cell: (a) => (
                <span className="block max-w-[18rem] truncate text-muted-foreground" title={a.reason}>
                    {a.reason}
                </span>
            ),
            csv: (a) => a.reason,
        },
        {
            id: "status",
            header: "Status",
            cell: (a) => (
                <div className="py-1">
                    <StatusBadge tone={STATUS_TONE[a.status]} dot>
                        {cap(a.status)}
                    </StatusBadge>
                    {a.review_notes && a.status !== "pending" && (
                        <div className="mt-1 max-w-xs truncate text-xs text-muted-foreground" title={a.review_notes}>
                            "{a.review_notes}"
                        </div>
                    )}
                </div>
            ),
            csv: (a) => a.status,
        },
        {
            id: "created",
            header: "Submitted",
            cell: (a) => (
                <span className="whitespace-nowrap tabular-nums text-muted-foreground">
                    {new Date(a.created_at).toLocaleDateString()}
                </span>
            ),
            csv: (a) => a.created_at,
        },
        {
            id: "reviewed",
            header: "Reviewed",
            defaultHidden: true,
            cell: (a) =>
                a.reviewed_at ? (
                    <span className="whitespace-nowrap tabular-nums text-muted-foreground">
                        {new Date(a.reviewed_at).toLocaleDateString()}
                    </span>
                ) : (
                    <span className="text-subtle-foreground">Not yet</span>
                ),
            csv: (a) => a.reviewed_at ?? "",
        },
        {
            id: "actions",
            header: "",
            align: "right",
            className: "w-16",
            cell: (a) => {
                const canReview = a.status === "pending";
                return (
                    <div className="inline-flex items-center gap-0.5 whitespace-nowrap">
                        <Button
                            size="icon-sm"
                            variant="ghost"
                            disabled={!canReview}
                            title="Approve"
                            aria-label="Approve"
                            onClick={(e) => {
                                e.stopPropagation();
                                setReviewing({ appeal: a, mode: "approve" });
                            }}
                        >
                            <CheckCircle2 className={cn("size-4", canReview && TONE_TEXT.success)} />
                        </Button>
                        <Button
                            size="icon-sm"
                            variant="ghost"
                            disabled={!canReview}
                            title="Reject"
                            aria-label="Reject"
                            onClick={(e) => {
                                e.stopPropagation();
                                setReviewing({ appeal: a, mode: "reject" });
                            }}
                        >
                            <XCircle className={cn("size-4", canReview && TONE_TEXT.danger)} />
                        </Button>
                    </div>
                );
            },
        },
    ];

    return (
        <>
            <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(340px,400px)] xl:items-start">
                <div className="min-w-0">
                    <div className="mb-3 flex flex-wrap items-center gap-2">
                        <Segmented
                            value={status}
                            onChange={setStatus}
                            options={STATUS_OPTIONS}
                            ariaLabel="Appeal status"
                        />
                    </div>
                    <DataTable
                        columns={columns}
                        rows={rows}
                        getRowId={(a) => a.id}
                        loading={isLoading}
                        error={error}
                        onRetry={() => refetch()}
                        onRowClick={select}
                        selectedRowId={selected?.id ?? null}
                        errorTitle="Failed to load appeals"
                        storageKey="admin.warmup-appeals"
                        csvName="warmbly-warmup-appeals"
                        noun="appeals"
                        emptyTitle="No appeals"
                        emptyHint={
                            status === "pending"
                                ? "No pending appeals to review."
                                : "No appeals match this status."
                        }
                        pager={{
                            canPrev: pager.canPrev,
                            canNext: !!data?.pagination?.has_more,
                            onPrev: pager.prev,
                            onNext: () => pager.next(data?.pagination?.next_cursor),
                            page: pager.page,
                            shown: rows.length,
                            total: data?.pagination?.total ?? null,
                        }}
                    />
                </div>

                <div ref={detailRef} className="scroll-mt-16 xl:sticky xl:top-16">
                    <AppealDetail appeal={selected} loading={isLoading} empty={!isLoading && rows.length === 0} status={status} />
                </div>
            </div>

            {reviewing && (
                <ReviewAppealDialog
                    appeal={reviewing.appeal}
                    mode={reviewing.mode}
                    open
                    onOpenChange={(v) => !v && setReviewing(null)}
                    onDone={() => setReviewing(null)}
                />
            )}
        </>
    );
}

// The triage detail: who appealed, why, what happened, and the decision.
function AppealDetail({
    appeal,
    loading,
    empty,
    status,
}: {
    appeal: WarmupAppeal | null;
    loading: boolean;
    empty: boolean;
    status: AppealStatusFilter;
}) {
    const frame = "overflow-hidden surface-lit rounded-xl border border-border bg-card";

    if (!appeal) {
        if (loading) {
            return (
                <div className={cn(frame, "space-y-3 p-4")}>
                    <Skeleton className="h-4 w-2/3" />
                    <Skeleton className="h-3 w-1/3" />
                    <Skeleton className="h-20 w-full" />
                </div>
            );
        }
        return (
            <div className={frame}>
                <EmptyState
                    icon={empty && status === "pending" ? CheckCircle2 : Inbox}
                    title={empty && status === "pending" ? "Queue is clear" : "No appeal selected"}
                    hint={
                        empty && status === "pending"
                            ? "Every warmup appeal has been reviewed."
                            : "Select an appeal from the list to review it."
                    }
                />
            </div>
        );
    }

    const mailbox = mailboxOf(appeal);
    const reviewer = appeal.reviewed_by_user?.email;

    return (
        <div className={frame}>
            <div className="flex items-start justify-between gap-3 border-b border-border px-4 py-3">
                <div className="min-w-0">
                    <div className="text-xs text-muted-foreground">Warmup appeal</div>
                    <div className="mt-0.5 truncate text-[15px] font-semibold tracking-[-0.01em] text-foreground" title={mailbox}>
                        {mailbox}
                    </div>
                </div>
                <StatusBadge tone={STATUS_TONE[appeal.status]} dot className="mt-0.5">
                    {cap(appeal.status)}
                </StatusBadge>
            </div>

            <div className="px-4 py-1">
                <PropertyList>
                    <Property label="User">{appeal.user?.email ?? appeal.user_id}</Property>
                    <Property label="Submitted">
                        <span className="tabular-nums">{new Date(appeal.created_at).toLocaleString()}</span>
                    </Property>
                    {appeal.reviewed_at && (
                        <Property label="Reviewed">
                            <span className="tabular-nums">{new Date(appeal.reviewed_at).toLocaleString()}</span>
                        </Property>
                    )}
                    {reviewer && <Property label="Reviewed by">{reviewer}</Property>}
                </PropertyList>
            </div>

            <div className="border-t border-border px-4 py-3">
                <div className="text-xs font-medium text-muted-foreground">Appeal reason</div>
                <p className="mt-1.5 whitespace-pre-wrap break-words text-[13px] leading-relaxed text-foreground">
                    {appeal.reason}
                </p>
            </div>

            {appeal.status !== "pending" && appeal.review_notes && (
                <div className="border-t border-border px-4 py-3">
                    <div className="text-xs font-medium text-muted-foreground">Review notes</div>
                    <p className="mt-1.5 whitespace-pre-wrap break-words text-[13px] leading-relaxed text-foreground">
                        {appeal.review_notes}
                    </p>
                </div>
            )}

            {appeal.status === "pending" && <DecisionArea key={appeal.id} appeal={appeal} />}
        </div>
    );
}

function DecisionArea({ appeal }: { appeal: WarmupAppeal }) {
    const [notes, setNotes] = useState("");
    const mutation = useReviewAppeal(appeal, () => setNotes(""));
    const busy = mutation.isPending;
    const pendingMode = busy ? mutation.variables?.mode : undefined;
    const id = `decision-notes-${appeal.id}`;
    const confirm = useConfirm();

    // Named confirm, so a repeat click never lands on the next appeal unseen.
    async function decide(mode: "approve" | "reject") {
        const mailbox = mailboxOf(appeal);
        const ok = await confirm(
            mode === "approve"
                ? {
                      title: `Approve the appeal for ${mailbox}?`,
                      description: "The mailbox is unblocked and re-admitted to the warmup pool.",
                      confirmLabel: "Approve & unblock",
                  }
                : {
                      title: `Reject the appeal for ${mailbox}?`,
                      description: "The mailbox stays blocked from warmup.",
                      confirmLabel: "Reject",
                      destructive: true,
                  },
        );
        if (ok) mutation.mutate({ mode, notes });
    }

    return (
        <div className="border-t border-border bg-muted/30 px-4 py-3.5">
            <Label htmlFor={id} className="text-xs font-medium text-muted-foreground">
                Review notes (optional)
            </Label>
            <Textarea
                id={id}
                placeholder="Optional: why the mailbox is being re-admitted, or why the appeal is rejected"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                className="mt-1.5 min-h-20 bg-card"
            />
            <div className="mt-3 flex flex-wrap items-center gap-2">
                <Button size="sm" disabled={busy} onClick={() => decide("approve")}>
                    <ShieldCheck />
                    {pendingMode === "approve" ? "Working…" : "Approve & unblock"}
                </Button>
                <Button size="sm" variant="outline" disabled={busy} onClick={() => decide("reject")}>
                    <XCircle className={TONE_TEXT.danger} />
                    {pendingMode === "reject" ? "Working…" : "Reject"}
                </Button>
            </div>
            <p className="mt-2.5 text-xs leading-relaxed text-muted-foreground">
                Approving unblocks the mailbox and re-admits it to the warmup pool. Rejecting keeps it blocked.
                Notes are recorded for audit and may be shown to the user.
            </p>
        </div>
    );
}

function ReviewAppealDialog({
    appeal,
    mode,
    open,
    onOpenChange,
    onDone,
}: {
    appeal: WarmupAppeal;
    mode: Mode;
    open: boolean;
    onOpenChange: (v: boolean) => void;
    onDone: () => void;
}) {
    const [notes, setNotes] = useState("");
    const mailbox = mailboxOf(appeal);
    const mutation = useReviewAppeal(appeal, onDone);

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {mode === "approve" ? "Approve appeal" : "Reject appeal"}
                    </DialogTitle>
                    <DialogDescription>
                        {mode === "approve" ? (
                            <>
                                Approving unblocks{" "}
                                <span className="font-medium text-foreground">{mailbox}</span> and
                                re-admits it to the warmup pool. Notes are recorded
                                for audit.
                            </>
                        ) : (
                            <>
                                Rejecting keeps{" "}
                                <span className="font-medium text-foreground">{mailbox}</span>{" "}
                                blocked. Notes are recorded for audit and may be
                                shown to the user.
                            </>
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div className="rounded-md border border-border bg-muted/40 px-3 py-2.5">
                    <div className="text-xs font-medium text-muted-foreground">Appeal reason</div>
                    <p className="mt-1 whitespace-pre-wrap break-words text-[13px] leading-relaxed text-foreground">
                        {appeal.reason}
                    </p>
                </div>
                <div className="space-y-1.5">
                    <Label htmlFor="notes" className="text-xs font-medium text-muted-foreground">
                        Review notes (optional)
                    </Label>
                    <Textarea
                        id="notes"
                        placeholder={
                            mode === "approve"
                                ? "Optional: why the mailbox is being re-admitted"
                                : "Optional: why the appeal is being rejected"
                        }
                        value={notes}
                        onChange={(e) => setNotes(e.target.value)}
                        autoFocus
                    />
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => mutation.mutate({ mode, notes })}
                        disabled={mutation.isPending}
                        variant={mode === "approve" ? "default" : "destructive"}
                    >
                        {mutation.isPending
                            ? "Working…"
                            : mode === "approve"
                              ? "Approve & unblock"
                              : "Reject"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

function BlockedTab() {
    const pager = useCursorPager();
    const [unblocking, setUnblocking] = useState<AdminBlockedAccount | null>(
        null,
    );

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "blocked", pager.cursor],
        queryFn: () => listBlockedWarmupAccounts(pager.cursor),
        placeholderData: keepPreviousData,
        staleTime: 10_000,
    });

    const rows = data?.data ?? [];

    const columns: Column<AdminBlockedAccount>[] = [
        {
            id: "mailbox",
            header: "Mailbox",
            cell: (a) => <span className="font-medium text-foreground">{a.email}</span>,
            csv: (a) => a.email,
        },
        {
            id: "owner",
            header: "Owner",
            cell: (a) => (
                <span className="text-muted-foreground">{a.user?.email ?? a.user_id}</span>
            ),
            csv: (a) => a.user?.email ?? a.user_id,
        },
        {
            id: "reason",
            header: "Reason",
            cell: (a) => (
                <span
                    className="block max-w-md truncate"
                    title={a.block_reason}
                >
                    {a.block_reason}
                </span>
            ),
            csv: (a) => a.block_reason,
        },
        {
            id: "blocked",
            header: "Blocked",
            cell: (a) => (
                <span className="whitespace-nowrap tabular-nums text-muted-foreground">
                    {new Date(a.blocked_at).toLocaleDateString()}
                </span>
            ),
            csv: (a) => a.blocked_at,
        },
        {
            id: "appeal",
            header: "Appeal",
            cell: (a) =>
                a.has_appeal ? (
                    <StatusBadge tone={STATUS_TONE[a.appeal_status ?? "pending"]} dot>
                        {cap(a.appeal_status ?? "pending")}
                    </StatusBadge>
                ) : (
                    <span className="text-subtle-foreground">None</span>
                ),
            csv: (a) => (a.has_appeal ? (a.appeal_status ?? "pending") : ""),
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (a) => (
                <Button size="xs" variant="outline" onClick={() => setUnblocking(a)}>
                    <ShieldCheck className={TONE_TEXT.success} /> Unblock
                </Button>
            ),
        },
    ];

    return (
        <>
            <DataTable
                columns={columns}
                rows={rows}
                getRowId={(a) => a.id}
                loading={isLoading}
                error={error}
                onRetry={() => refetch()}
                errorTitle="Failed to load blocked mailboxes"
                storageKey="admin.warmup-blocked"
                csvName="warmbly-warmup-blocked"
                noun="mailboxes"
                emptyTitle="No blocked mailboxes"
                emptyHint="No mailboxes are currently blocked from warmup."
                pager={{
                    canPrev: pager.canPrev,
                    canNext: !!data?.pagination?.has_more,
                    onPrev: pager.prev,
                    onNext: () => pager.next(data?.pagination?.next_cursor),
                    page: pager.page,
                    shown: rows.length,
                    total: data?.pagination?.total ?? null,
                }}
            />

            {unblocking && (
                <UnblockDialog
                    account={unblocking}
                    open
                    onOpenChange={(v) => !v && setUnblocking(null)}
                    onDone={() => setUnblocking(null)}
                />
            )}
        </>
    );
}

function UnblockDialog({
    account,
    open,
    onOpenChange,
    onDone,
}: {
    account: AdminBlockedAccount;
    open: boolean;
    onOpenChange: (v: boolean) => void;
    onDone: () => void;
}) {
    const qc = useQueryClient();
    const mutation = useMutation({
        mutationFn: () => unblockWarmupAccount(account.id),
        onSuccess: () => {
            toast.success("Mailbox unblocked");
            qc.invalidateQueries({ queryKey: ["admin", "warmup"] });
            onDone();
        },
        onError: (err: Error) => toast.error(err.message || "Failed to unblock"),
    });

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Unblock mailbox</DialogTitle>
                    <DialogDescription>
                        Unblock <span className="font-medium text-foreground">{account.email}</span>{" "}
                        and re-admit it to the warmup pool? This bypasses any open
                        appeal and is recorded for audit.
                    </DialogDescription>
                </DialogHeader>
                {account.has_appeal && (
                    <Callout tone="warning" icon={AlertTriangle}>
                        This mailbox has an open appeal
                        {account.appeal_status
                            ? ` (${account.appeal_status})`
                            : ""}
                        . Unblocking here does not record an appeal decision.
                        Prefer approving the appeal if you want it tracked as a
                        review outcome.
                    </Callout>
                )}
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => mutation.mutate()}
                        disabled={mutation.isPending}
                    >
                        {mutation.isPending ? "Working…" : "Unblock"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
