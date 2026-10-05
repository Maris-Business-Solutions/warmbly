// Limit-increase request queue: left filter rail + server-driven sortable,
// cursor-paged table (mirrors OrganizationsPage). Approving writes the
// corresponding override on the org via the same SetLimitOverrides path the
// manual editor uses; rejecting stamps the row with required review notes.

import { useEffect, useState } from "react";
import {
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { ArrowRight, Check, X } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Segmented, StatusBadge } from "@/components/ui/kit";
import { TONE_TEXT, type Tone } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Explorer,
    FilterGroup,
    SearchFilter,
    SelectFilter,
    ToggleFilter,
    DateRangeFilter,
    NumberRangeFilter,
} from "@/components/data/Explorer";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useCursorPager } from "@/lib/useCursorPager";
import {
    emptyRange,
    rangeActive,
    rangeWithin,
    rangeAfter,
    rangeBefore,
    type DateRange,
} from "@/lib/dateRange";
import {
    approveLimitRequest,
    listLimitRequests,
    rejectLimitRequest,
} from "@/lib/api/client/admin/limitRequests";
import type {
    AdminLimitRequestSearch,
    LimitIncreaseRequest,
    LimitRequestStatus,
} from "@/lib/api/models/admin";

type StatusFilter = LimitRequestStatus | "all";

const STATUS_TONE: Record<LimitRequestStatus, Tone> = {
    pending: "warning",
    approved: "success",
    rejected: "danger",
    cancelled: "neutral",
};

const FIELD_LABEL: Record<string, string> = {
    max_email_accounts: "Mailboxes",
    max_campaigns: "Campaigns (lifetime)",
    max_active_campaigns: "Active campaigns",
    max_team_members: "Team members",
    max_contacts: "Contacts",
    daily_campaign_limit: "Daily sends",
};

// Status views for the queue's header switcher.
const STATUS_VIEWS: { value: StatusFilter; label: string }[] = [
    { value: "pending", label: "Pending" },
    { value: "approved", label: "Approved" },
    { value: "rejected", label: "Rejected" },
    { value: "cancelled", label: "Cancelled" },
    { value: "all", label: "All" },
];

const FIELD_OPTIONS = [
    { value: "any", label: "Any field" },
    ...Object.entries(FIELD_LABEL).map(([value, label]) => ({ value, label })),
];

export default function LimitRequestsPage() {
    const nav = useNavigate();
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState<StatusFilter>("pending");
    const [field, setField] = useState("");
    const [reviewed, setReviewed] = useState(false);
    const [unreviewed, setUnreviewed] = useState(false);
    const [reqMin, setReqMin] = useState<number | undefined>();
    const [reqMax, setReqMax] = useState<number | undefined>();
    const [curMin, setCurMin] = useState<number | undefined>();
    const [curMax, setCurMax] = useState<number | undefined>();
    const [submitted, setSubmitted] = useState<DateRange>(emptyRange);
    const [reviewedRange, setReviewedRange] = useState<DateRange>(emptyRange);

    const [sort, setSort] = useState<{ by: string; desc: boolean }>({ by: "", desc: true });
    const pager = useCursorPager();
    const { reset } = pager;

    const [reviewing, setReviewing] = useState<{
        req: LimitIncreaseRequest;
        mode: "approve" | "reject";
    } | null>(null);

    const filterKey = JSON.stringify({
        query, status, field, reviewed, unreviewed,
        reqMin, reqMax, curMin, curMax, submitted, reviewedRange, sort,
    });

    useEffect(() => {
        reset();
    }, [filterKey, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "limit-requests", filterKey, pager.cursor],
        queryFn: () =>
            listLimitRequests({
                q: query.trim() || undefined,
                status: status === "all" ? "" : status,
                field: field || undefined,
                reviewed: reviewed || undefined,
                unreviewed: unreviewed || undefined,
                requested_min: reqMin,
                requested_max: reqMax,
                current_effective_min: curMin,
                current_effective_max: curMax,
                submitted_within: rangeWithin(submitted),
                submitted_after: rangeAfter(submitted),
                submitted_before: rangeBefore(submitted),
                reviewed_after: rangeAfter(reviewedRange),
                reviewed_before: rangeBefore(reviewedRange),
                limit: 50,
                cursor: pager.cursor,
                sort_by: sort.by ? (sort.by as AdminLimitRequestSearch["sort_by"]) : undefined,
                sort_desc: sort.by ? sort.desc : undefined,
            }),
        staleTime: 30_000,
        placeholderData: keepPreviousData,
    });

    const rows = data?.data ?? [];

    const bools = [reviewed, unreviewed];
    const ranges = [[reqMin, reqMax], [curMin, curMax]];
    const activeCount =
        (query ? 1 : 0) +
        (status !== "pending" ? 1 : 0) +
        (field ? 1 : 0) +
        bools.filter(Boolean).length +
        ranges.filter(([a, b]) => a !== undefined || b !== undefined).length +
        [submitted, reviewedRange].filter(rangeActive).length +
        (sort.by ? 1 : 0);

    function resetAll() {
        setQuery("");
        setStatus("pending");
        setField("");
        setReviewed(false);
        setUnreviewed(false);
        setReqMin(undefined);
        setReqMax(undefined);
        setCurMin(undefined);
        setCurMax(undefined);
        setSubmitted(emptyRange);
        setReviewedRange(emptyRange);
        setSort({ by: "", desc: true });
    }

    const columns: Column<LimitIncreaseRequest>[] = [
        {
            id: "workspace",
            header: "Workspace",
            sortable: true,
            sortKey: "org_name",
            cell: (r) => (
                <div className="min-w-0 py-1.5 leading-tight">
                    <Link
                        to={`/organizations/${r.organization_id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="font-medium text-foreground hover:text-[var(--admin-accent-strong)] hover:underline"
                    >
                        {r.organization?.name ?? r.organization_id}
                    </Link>
                    {r.organization?.slug && (
                        <div className="mt-0.5 font-mono text-[11px] text-subtle-foreground">{r.organization.slug}</div>
                    )}
                </div>
            ),
            csv: (r) => r.organization?.name ?? r.organization_id,
        },
        {
            id: "requester",
            header: "Requester",
            cell: (r) => (
                <span className="text-muted-foreground">{r.submitted_by_user?.email ?? r.submitted_by}</span>
            ),
            csv: (r) => r.submitted_by_user?.email ?? r.submitted_by,
        },
        {
            id: "field",
            header: "Field",
            sortable: true,
            sortKey: "field",
            cell: (r) => <span className="whitespace-nowrap text-foreground">{FIELD_LABEL[r.field] ?? r.field}</span>,
            csv: (r) => FIELD_LABEL[r.field] ?? r.field,
        },
        {
            id: "current",
            header: "Current",
            align: "right",
            sortable: true,
            sortKey: "current_effective",
            cell: (r) => (
                <span className="tabular-nums text-muted-foreground">{r.current_effective.toLocaleString()}</span>
            ),
            csv: (r) => r.current_effective,
        },
        {
            id: "requested",
            header: "Requested",
            align: "right",
            sortable: true,
            sortKey: "requested",
            cell: (r) => (
                <span className="whitespace-nowrap tabular-nums font-medium text-foreground">
                    {r.requested.toLocaleString()}
                    <span className={cn("ml-1.5 text-xs font-normal", TONE_TEXT.success)}>
                        +{(r.requested - r.current_effective).toLocaleString()}
                    </span>
                </span>
            ),
            csv: (r) => r.requested,
        },
        {
            id: "reason",
            header: "Reason",
            cell: (r) => (
                <span className="block max-w-md truncate text-muted-foreground" title={r.reason}>
                    {r.reason}
                </span>
            ),
            csv: (r) => r.reason,
        },
        {
            id: "status",
            header: "Status",
            sortable: true,
            sortKey: "status",
            cell: (r) => (
                <div className="py-1.5">
                    <StatusBadge tone={STATUS_TONE[r.status]} dot>
                        {r.status}
                    </StatusBadge>
                    {r.review_notes && r.status !== "pending" && (
                        <div className="mt-1 max-w-xs truncate text-xs text-muted-foreground" title={r.review_notes}>
                            "{r.review_notes}"
                        </div>
                    )}
                </div>
            ),
            csv: (r) => r.status,
        },
        {
            id: "submitted",
            header: "Submitted",
            sortable: true,
            sortKey: "submitted_at",
            cell: (r) => (
                <span className="whitespace-nowrap tabular-nums text-muted-foreground">{new Date(r.submitted_at).toLocaleDateString()}</span>
            ),
            csv: (r) => r.submitted_at,
        },
        {
            id: "reviewed",
            header: "Reviewed",
            sortable: true,
            sortKey: "reviewed_at",
            defaultHidden: true,
            cell: (r) => (
                <span className="whitespace-nowrap tabular-nums text-muted-foreground">
                    {r.reviewed_at ? new Date(r.reviewed_at).toLocaleDateString() : "—"}
                </span>
            ),
            csv: (r) => r.reviewed_at ?? "",
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (r) => {
                const canReview = r.status === "pending";
                return (
                    <div className="inline-flex items-center gap-1 whitespace-nowrap">
                        <Button
                            size="xs"
                            variant="outline"
                            disabled={!canReview}
                            title={canReview ? "Approve this request" : "Already reviewed"}
                            onClick={(e) => {
                                e.stopPropagation();
                                setReviewing({ req: r, mode: "approve" });
                            }}
                        >
                            <Check className={cn("size-3", TONE_TEXT.success)} /> Approve
                        </Button>
                        <Button
                            size="xs"
                            variant="outline"
                            disabled={!canReview}
                            title={canReview ? "Reject this request" : "Already reviewed"}
                            onClick={(e) => {
                                e.stopPropagation();
                                setReviewing({ req: r, mode: "reject" });
                            }}
                        >
                            <X className={cn("size-3", TONE_TEXT.danger)} /> Reject
                        </Button>
                    </div>
                );
            },
        },
    ];

    return (
        <div>
            <PageHeader
                title="Limit-increase requests"
                meta={
                    data?.pagination?.total != null ? (
                        <span className="text-[13px] tabular-nums text-subtle-foreground">
                            {data.pagination.total.toLocaleString()}
                        </span>
                    ) : undefined
                }
                description="Customer-submitted requests for more capacity than their plan or product hard cap allows. Approving rewrites the per-org override; rejecting stamps the row with notes."
            >
                <Segmented
                    ariaLabel="Status"
                    value={status}
                    onChange={setStatus}
                    options={STATUS_VIEWS}
                />
            </PageHeader>
            <Explorer
                activeCount={activeCount}
                onReset={resetAll}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="Org, requester, or reason…" />
                        </FilterGroup>
                        <FilterGroup label="Field">
                            <SelectFilter
                                value={field || "any"}
                                onChange={(v) => setField(v === "any" ? "" : v)}
                                options={FIELD_OPTIONS}
                                placeholder="Any field"
                            />
                        </FilterGroup>
                        <FilterGroup label="Review state">
                            <div className="flex flex-col gap-2">
                                <ToggleFilter checked={reviewed} onChange={setReviewed} label="Reviewed" />
                                <ToggleFilter checked={unreviewed} onChange={setUnreviewed} label="Awaiting review" />
                            </div>
                        </FilterGroup>
                        <FilterGroup label="Requested amount">
                            <NumberRangeFilter min={reqMin} max={reqMax} onMinChange={setReqMin} onMaxChange={setReqMax} />
                        </FilterGroup>
                        <FilterGroup label="Current effective">
                            <NumberRangeFilter min={curMin} max={curMax} onMinChange={setCurMin} onMaxChange={setCurMax} />
                        </FilterGroup>
                        <FilterGroup label="Submitted">
                            <DateRangeFilter value={submitted} onChange={setSubmitted} />
                        </FilterGroup>
                        <FilterGroup label="Reviewed">
                            <DateRangeFilter value={reviewedRange} onChange={setReviewedRange} mode="custom" />
                        </FilterGroup>
                    </>
                }
            >
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(r) => r.id}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load limit requests"
                    onRowClick={(r) => nav(`/organizations/${r.organization_id}`)}
                    sort={sort.by ? sort : undefined}
                    onSortChange={setSort}
                    storageKey="admin.limit-requests"
                    csvName="warmbly-limit-requests"
                    noun="requests"
                    emptyTitle="No limit requests"
                    emptyHint="No requests match these filters."
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
            </Explorer>

            {reviewing && (
                <ReviewDialog
                    req={reviewing.req}
                    mode={reviewing.mode}
                    open
                    onOpenChange={(v) => !v && setReviewing(null)}
                />
            )}
        </div>
    );
}

function ReviewDialog({
    req,
    mode,
    open,
    onOpenChange,
}: {
    req: LimitIncreaseRequest;
    mode: "approve" | "reject";
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [notes, setNotes] = useState("");
    const mutation = useMutation({
        mutationFn: () =>
            mode === "approve"
                ? approveLimitRequest(req.id, notes)
                : rejectLimitRequest(req.id, notes),
        onSuccess: () => {
            toast.success(`Request ${mode === "approve" ? "approved" : "rejected"}`);
            qc.invalidateQueries({ queryKey: ["admin", "limit-requests"] });
            qc.invalidateQueries({ queryKey: ["admin", "organizations", req.organization_id] });
            onOpenChange(false);
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    const fieldLabel = FIELD_LABEL[req.field] ?? req.field;

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {mode === "approve" ? "Approve request" : "Reject request"}
                    </DialogTitle>
                    <DialogDescription>
                        {mode === "approve" ? (
                            <>
                                Approving raises <span className="font-medium text-foreground">{fieldLabel}</span> for{" "}
                                <span className="font-medium text-foreground">
                                    {req.organization?.name ?? req.organization_id}
                                </span>{" "}
                                from {req.current_effective.toLocaleString()} to{" "}
                                {req.requested.toLocaleString()}. This writes the
                                corresponding override on the org and is auditable.
                            </>
                        ) : (
                            <>
                                Rejecting the request stamps it with your notes for the
                                customer to see. The org keeps its current effective
                                limit ({req.current_effective.toLocaleString()}).
                            </>
                        )}
                    </DialogDescription>
                </DialogHeader>
                <div className="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3.5 py-2.5 text-[13px]">
                    <div className="min-w-0 flex-1">
                        <div className="truncate font-medium text-foreground">
                            {req.organization?.name ?? req.organization_id}
                        </div>
                        <div className="text-xs text-muted-foreground">{fieldLabel}</div>
                    </div>
                    <div className="flex shrink-0 items-center gap-2 tabular-nums">
                        <span className="text-muted-foreground">{req.current_effective.toLocaleString()}</span>
                        <ArrowRight className="size-3.5 text-subtle-foreground" />
                        <span className="font-medium text-foreground">{req.requested.toLocaleString()}</span>
                    </div>
                </div>
                {req.reason && (
                    <p className="-mt-1 border-l-2 border-border pl-3 text-[12.5px] leading-relaxed text-muted-foreground">
                        {req.reason}
                    </p>
                )}
                <div className="space-y-1.5">
                    <Label htmlFor="notes" className="text-xs font-medium text-muted-foreground">
                        Notes <span className="font-normal text-subtle-foreground">{mode === "reject" ? "(required)" : "(optional)"}</span>
                    </Label>
                    <Input
                        id="notes"
                        placeholder={
                            mode === "approve"
                                ? "Optional: business reason for the bump"
                                : "Required: tell the customer why"
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
                        onClick={() => {
                            if (mode === "reject" && notes.trim() === "") {
                                toast.error("Notes are required when rejecting");
                                return;
                            }
                            mutation.mutate();
                        }}
                        disabled={mutation.isPending}
                        variant={mode === "approve" ? "default" : "destructive"}
                    >
                        {mutation.isPending
                            ? "Working…"
                            : mode === "approve"
                            ? "Approve"
                            : "Reject"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
