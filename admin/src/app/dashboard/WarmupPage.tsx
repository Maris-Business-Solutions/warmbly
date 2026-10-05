// Warmup pool admin. The platform's most critical safety surface per
// CLAUDE.md: shared paid-pool reputation is more valuable than maximum
// access for one risky mailbox, so the page foregrounds:
//
//   1. Aggregate health (counts by state, avg spam-score & placement rate)
//   2. Per-pool participant + blocked counts
//   3. Blocked account list with unblock action
//   4. Pending appeals with one-click approve/reject
//
// Nothing here polls: the realtime spine's warmup group invalidates
// ["admin","warmup"] on ACCOUNT and WARMUP events. The action
// history tab (?tab=) comes from /admin/warmup/actions.

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { toast } from "sonner";
import {
    Activity,
    AlertTriangle,
    CheckCircle2,
    Flame,
    History,
    Inbox,
    LayoutDashboard,
    ShieldCheck,
    ShieldOff,
    XCircle,
} from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageTabs } from "@/components/layout/PageTabs";
import { StateLegend } from "@/components/StateLegend";
import { MAILBOX_HEALTH_LEGEND } from "@/lib/legends";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, Panel, Section, Stat, StatGrid, StatusBadge } from "@/components/ui/kit";
import { ErrorState } from "@/components/ErrorState";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    approveAppeal,
    getWarmupHealthSummary,
    listBlockedWarmupAccounts,
    listWarmupActions,
    listWarmupAppeals,
    listWarmupPools,
    rejectAppeal,
    unblockWarmupAccount,
} from "@/lib/api/client/admin/warmup";
import type {
    AdminBlockedAccount,
    WarmupAppeal,
} from "@/lib/api/models/admin";
import { TONE_DOT, TONE_TEXT, type Tone } from "@/lib/tones";
import { cn } from "@/lib/utils";

// Hand-rolled table chrome, matching DataTable.
const TABLE = "w-full border-collapse text-[13px]";
const TH = "h-9 whitespace-nowrap px-3 text-left text-xs font-medium text-muted-foreground first:pl-4 last:pr-4";
const TR = "h-10 border-b border-border/70 transition-colors last:border-0 hover:bg-accent/50";
const TD = "px-3 align-middle first:pl-4 last:pr-4";

// Health states in severity order, for the distribution bar.
const STATE_TONE: Record<string, Tone> = {
    healthy: "success",
    watch: "warning",
    throttled: "orange",
    quarantined: "danger",
    blocked: "danger",
};
const STATE_ORDER = ["healthy", "watch", "throttled", "quarantined", "blocked"];

const cap = (s: string) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : s);

function TableFrame({ children }: { children: React.ReactNode }) {
    return (
        <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
            <div className="overflow-x-auto">{children}</div>
        </div>
    );
}

function EmptyFrame({ icon, title, hint }: { icon: typeof Inbox; title: string; hint?: string }) {
    return (
        <div className="surface-lit rounded-xl border border-border bg-card">
            <EmptyState icon={icon} title={title} hint={hint} className="py-10" />
        </div>
    );
}

const TABS = [
    { id: "overview", label: "Overview", icon: LayoutDashboard },
    { id: "actions", label: "Admin actions", icon: History },
] as const;

type TabId = (typeof TABS)[number]["id"];

function isTab(v: string | null): v is TabId {
    return TABS.some((t) => t.id === v);
}

export default function WarmupPage() {
    const [params, setParams] = useSearchParams();
    const raw = params.get("tab");
    const tab: TabId = isTab(raw) ? raw : "overview";

    function setTab(next: string) {
        setParams(
            (p) => {
                p.set("tab", next);
                return p;
            },
            { replace: true },
        );
    }

    return (
        <div>
            <PageHeader
                title="Warmup pools"
                description="Pool health, blocked mailboxes, and pending appeals. Shared paid-pool reputation matters more than any single mailbox, so quarantine early."
            >
                <StateLegend label="Health states explained" entries={MAILBOX_HEALTH_LEGEND} />
            </PageHeader>

            <PageTabs tabs={[...TABS]} value={tab} onChange={setTab} />

            {tab === "actions" && <ActionsTab />}
            {tab === "overview" && <Overview />}
        </div>
    );
}

function Overview() {
    return (
        <div>
            <HealthSummary />

            <Section title="Pools" className="mt-8 first:mt-8">
                <PoolsList />
            </Section>

            <Section title="Blocked mailboxes">
                <BlockedAccounts />
            </Section>

            <Section title="Appeals queue">
                <AppealsQueue />
            </Section>
        </div>
    );
}

function HealthSummary() {
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "health"],
        queryFn: getWarmupHealthSummary,
    });

    if (isLoading) {
        return (
            <StatGrid>
                {["Total participants", "At risk", "Spam placement", "Blocked"].map((l) => (
                    <Stat key={l} label={l} value={null} loading />
                ))}
            </StatGrid>
        );
    }
    if (error) {
        return <ErrorState error={error} title="Failed to load warmup health" onRetry={() => refetch()} />;
    }
    if (!data) return null;

    // Guard every field: a partial response must degrade gracefully, never
    // throw and blank the whole tab.
    //
    // `avg_spam_placement_rate` is ALREADY a percent (the backend computes
    // placements/sent*100), so we render it directly, no second *100.
    const placement = data.avg_spam_placement_rate ?? 0;
    const spamPct = placement.toFixed(1);
    const placementTone: Tone = placement >= 20 ? "danger" : placement >= 10 ? "warning" : "success";
    const atRisk = data.at_risk_count ?? 0;
    const blocked = data.blocked_count ?? 0;

    // `spam_placement_by_provider` is a Record of RAW COUNTS (backend does
    // COUNT(*)), not rates. Sort worst-first by count so the riskiest
    // mailbox-provider surface is the first thing an investigator sees.
    const byProvider = Object.entries(data.spam_placement_by_provider ?? {}).sort(
        (a, b) => b[1] - a[1],
    );

    const byState = Object.entries(data.by_state ?? {}).sort(
        (a, b) => rank(a[0]) - rank(b[0]),
    );

    return (
        <div>
            <StatGrid>
                <Stat
                    icon={Activity}
                    label="Total participants"
                    value={(data.total_participants ?? 0).toLocaleString()}
                    sub="across all pools"
                />
                <Stat
                    icon={AlertTriangle}
                    label="At risk"
                    value={atRisk.toLocaleString()}
                    sub={`avg health score ${(data.avg_health_score ?? 0).toFixed(1)}`}
                    tone={atRisk > 0 ? "warning" : undefined}
                />
                <Stat icon={Flame} label="Spam placement" value={`${spamPct}%`} sub="avg across pool" tone={placementTone} />
                <Stat
                    icon={ShieldOff}
                    label="Blocked"
                    value={blocked.toLocaleString()}
                    sub="quarantined or hard-blocked"
                    tone={blocked > 0 ? "danger" : undefined}
                />
            </StatGrid>

            {(byState.length > 0 || byProvider.length > 0) && (
                <div className={cn("mt-4 grid gap-4", byState.length > 0 && byProvider.length > 0 && "lg:grid-cols-2")}>
                    {byState.length > 0 && <StateDistribution entries={byState} />}
                    {byProvider.length > 0 && <ProviderPlacements entries={byProvider} />}
                </div>
            )}
        </div>
    );
}

function rank(state: string): number {
    const i = STATE_ORDER.indexOf(state);
    return i === -1 ? STATE_ORDER.length : i;
}

// One stacked bar of participants by health state, with a legend below.
function StateDistribution({ entries }: { entries: [string, number][] }) {
    const total = entries.reduce((n, [, v]) => n + (v ?? 0), 0);
    return (
        <Panel title="Participants by state" description={`${total.toLocaleString()} across all pools`}>
            <div className="flex h-2 w-full gap-px overflow-hidden rounded-full bg-muted">
                {total > 0 &&
                    entries.map(([state, count]) =>
                        count > 0 ? (
                            <div
                                key={state}
                                title={`${state}: ${count.toLocaleString()}`}
                                className={cn("h-full", TONE_DOT[STATE_TONE[state] ?? "neutral"])}
                                style={{ width: `${(count / total) * 100}%` }}
                            />
                        ) : null,
                    )}
            </div>
            <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-1.5 sm:grid-cols-3">
                {entries.map(([state, count]) => (
                    <div key={state} className="flex items-center justify-between gap-2 text-[13px]">
                        <dt className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
                            <span className={cn("size-2 shrink-0 rounded-full", TONE_DOT[STATE_TONE[state] ?? "neutral"])} />
                            <span className="truncate">{cap(state)}</span>
                        </dt>
                        <dd className="font-medium tabular-nums text-foreground">{(count ?? 0).toLocaleString()}</dd>
                    </div>
                ))}
            </dl>
        </Panel>
    );
}

// Raw spam-placement counts per provider, worst first, as bars against the max.
function ProviderPlacements({ entries }: { entries: [string, number][] }) {
    const max = Math.max(1, ...entries.map(([, c]) => c ?? 0));
    return (
        <Panel title="Spam placements by provider" description="Count of placements, worst first">
            <div className="space-y-2">
                {entries.map(([provider, count]) => (
                    <div key={provider} className="grid grid-cols-[minmax(5rem,9rem)_1fr_auto] items-center gap-3 text-[13px]">
                        <span className="truncate capitalize text-foreground">{provider}</span>
                        <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                            <div
                                className="h-full rounded-full bg-[var(--admin-accent)]"
                                style={{ width: `${((count ?? 0) / max) * 100}%` }}
                            />
                        </div>
                        <span className="min-w-8 text-right font-medium tabular-nums text-foreground">
                            {(count ?? 0).toLocaleString()}
                        </span>
                    </div>
                ))}
            </div>
        </Panel>
    );
}

function PoolsList() {
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "pools"],
        queryFn: listWarmupPools,
    });

    if (isLoading) return <Skeleton className="h-28 rounded-lg" />;
    if (error) {
        return <ErrorState error={error} title="Failed to load pools" onRetry={() => refetch()} />;
    }
    const pools = data ?? [];

    if (pools.length === 0) return <EmptyFrame icon={Inbox} title="No pools." />;

    return (
        <TableFrame>
            <table className={TABLE}>
                <thead>
                    <tr className="border-b border-border">
                        <th className={TH}>Pool</th>
                        <th className={cn(TH, "text-right")}>Total</th>
                        <th className={cn(TH, "text-right")}>Active</th>
                        <th className={cn(TH, "text-right")}>Blocked</th>
                    </tr>
                </thead>
                <tbody>
                    {pools.map((p) => (
                        <tr key={p.type} className={TR}>
                            <td className={TD}>
                                <StatusBadge tone={p.type === "premium" ? "strong" : "neutral"}>{cap(p.type)}</StatusBadge>
                            </td>
                            <td className={cn(TD, "text-right tabular-nums")}>{p.total_participants.toLocaleString()}</td>
                            <td className={cn(TD, "text-right tabular-nums", TONE_TEXT.success)}>
                                {p.active_participants.toLocaleString()}
                            </td>
                            <td
                                className={cn(
                                    TD,
                                    "text-right tabular-nums",
                                    p.blocked_count > 0 ? TONE_TEXT.danger : "text-muted-foreground",
                                )}
                            >
                                {p.blocked_count.toLocaleString()}
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </TableFrame>
    );
}

function BlockedAccounts() {
    const qc = useQueryClient();
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "blocked"],
        queryFn: () => listBlockedWarmupAccounts(),
    });

    const unblock = useMutation({
        mutationFn: (accountId: string) => unblockWarmupAccount(accountId),
        onSuccess: () => {
            toast.success("Mailbox unblocked");
            qc.invalidateQueries({ queryKey: ["admin", "warmup"] });
        },
        onError: (err: Error) => toast.error(err.message || "Failed to unblock"),
    });

    if (isLoading) return <Skeleton className="h-32 rounded-lg" />;
    if (error) {
        return <ErrorState error={error} title="Failed to load blocked mailboxes" onRetry={() => refetch()} />;
    }
    const rows = data?.data ?? [];

    if (rows.length === 0) {
        return <EmptyFrame icon={ShieldCheck} title="No mailboxes are currently blocked." />;
    }

    return (
        <TableFrame>
            <table className={TABLE}>
                <thead>
                    <tr className="border-b border-border">
                        <th className={TH}>Mailbox</th>
                        <th className={TH}>Owner</th>
                        <th className={TH}>Reason</th>
                        <th className={TH}>Blocked</th>
                        <th className={TH}>Appeal</th>
                        <th className={cn(TH, "text-right")}>Action</th>
                    </tr>
                </thead>
                <tbody>
                    {rows.map((a) => (
                        <BlockedRow key={a.id} account={a} onUnblock={() => unblock.mutate(a.id)} />
                    ))}
                </tbody>
            </table>
        </TableFrame>
    );
}

function BlockedRow({
    account,
    onUnblock,
}: {
    account: AdminBlockedAccount;
    onUnblock: () => void;
}) {
    return (
        <tr className={TR}>
            <td className={cn(TD, "font-medium text-foreground")}>{account.email}</td>
            <td className={cn(TD, "text-muted-foreground")}>{account.user?.email ?? account.user_id}</td>
            <td className={TD}>
                <span className="block max-w-sm truncate" title={account.block_reason}>
                    {account.block_reason}
                </span>
            </td>
            <td className={cn(TD, "whitespace-nowrap tabular-nums text-muted-foreground")}>
                {new Date(account.blocked_at).toLocaleDateString()}
            </td>
            <td className={TD}>
                {account.has_appeal ? (
                    <StatusBadge tone="warning" dot>
                        {cap(account.appeal_status ?? "pending")}
                    </StatusBadge>
                ) : (
                    <span className="text-subtle-foreground">None</span>
                )}
            </td>
            <td className={cn(TD, "text-right")}>
                <Button size="xs" variant="outline" onClick={onUnblock}>
                    <CheckCircle2 className={TONE_TEXT.success} />
                    Unblock
                </Button>
            </td>
        </tr>
    );
}

function AppealsQueue() {
    const qc = useQueryClient();
    const [reviewing, setReviewing] = useState<{
        appeal: WarmupAppeal;
        mode: "approve" | "reject";
    } | null>(null);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "appeals", "pending"],
        queryFn: () => listWarmupAppeals("pending"),
    });

    if (isLoading) return <Skeleton className="h-32 rounded-lg" />;
    if (error) {
        return <ErrorState error={error} title="Failed to load appeals" onRetry={() => refetch()} />;
    }
    const rows = data?.data ?? [];

    return (
        <>
            {rows.length === 0 ? (
                <EmptyFrame icon={Inbox} title="No pending appeals." />
            ) : (
                <TableFrame>
                    <table className={TABLE}>
                        <thead>
                            <tr className="border-b border-border">
                                <th className={TH}>Mailbox</th>
                                <th className={TH}>User</th>
                                <th className={TH}>Reason</th>
                                <th className={TH}>Submitted</th>
                                <th className={cn(TH, "text-right")}>Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {rows.map((a) => (
                                <tr key={a.id} className={TR}>
                                    <td className={cn(TD, "font-medium text-foreground")}>
                                        {a.email_account?.email ?? a.email_account_id}
                                    </td>
                                    <td className={cn(TD, "text-muted-foreground")}>{a.user?.email ?? a.user_id}</td>
                                    <td className={TD}>
                                        <span className="block max-w-md truncate" title={a.reason}>
                                            {a.reason}
                                        </span>
                                    </td>
                                    <td className={cn(TD, "whitespace-nowrap tabular-nums text-muted-foreground")}>
                                        {new Date(a.created_at).toLocaleDateString()}
                                    </td>
                                    <td className={cn(TD, "text-right")}>
                                        <div className="inline-flex items-center gap-1.5 whitespace-nowrap">
                                            <Button size="xs" variant="outline" onClick={() => setReviewing({ appeal: a, mode: "approve" })}>
                                                <CheckCircle2 className={TONE_TEXT.success} /> Approve
                                            </Button>
                                            <Button size="xs" variant="outline" onClick={() => setReviewing({ appeal: a, mode: "reject" })}>
                                                <XCircle className={TONE_TEXT.danger} /> Reject
                                            </Button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </TableFrame>
            )}

            {reviewing && (
                <ReviewAppealDialog
                    appeal={reviewing.appeal}
                    mode={reviewing.mode}
                    open
                    onOpenChange={(v) => !v && setReviewing(null)}
                    onDone={() => {
                        qc.invalidateQueries({ queryKey: ["admin", "warmup"] });
                        setReviewing(null);
                    }}
                />
            )}
        </>
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
    mode: "approve" | "reject";
    open: boolean;
    onOpenChange: (v: boolean) => void;
    onDone: () => void;
}) {
    const [notes, setNotes] = useState("");
    const mutation = useMutation({
        mutationFn: () =>
            mode === "approve"
                ? approveAppeal(appeal.id, { approved: true, notes })
                : rejectAppeal(appeal.id, { approved: false, notes }),
        onSuccess: () => {
            toast.success(`Appeal ${mode === "approve" ? "approved" : "rejected"}`);
            onDone();
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {mode === "approve" ? "Approve appeal" : "Reject appeal"}
                    </DialogTitle>
                    <DialogDescription>
                        {mode === "approve"
                            ? "Approving will unblock the mailbox and re-admit it to the pool. Notes are recorded for audit."
                            : "Rejecting keeps the mailbox blocked. Notes are recorded for audit and may be shown to the user."}
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-1.5">
                    <Label htmlFor="notes" className="text-xs font-medium text-muted-foreground">
                        Review notes
                    </Label>
                    <Input
                        id="notes"
                        placeholder="Brief justification (required)"
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
                            if (notes.trim() === "") {
                                toast.error("Notes are required");
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

// ---- admin action history ----

function ActionsTab() {
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "warmup", "actions"],
        queryFn: () => listWarmupActions(200),
    });
    const rows = data?.data ?? [];

    return (
        <div>
            <p className="mb-4 max-w-2xl text-[13px] leading-relaxed text-muted-foreground">
                Every manual block, unblock and appeal decision an admin made on a warmup mailbox, newest first.
            </p>
            {isLoading ? (
                <Skeleton className="h-40 w-full rounded-lg" />
            ) : error ? (
                <ErrorState error={error} title="Failed to load action history" onRetry={() => refetch()} />
            ) : rows.length === 0 ? (
                <EmptyFrame
                    icon={History}
                    title="No admin actions recorded yet"
                    hint="Blocking or unblocking a mailbox, or reviewing an appeal, writes a row here."
                />
            ) : (
                <TableFrame>
                    <table className={TABLE}>
                        <thead>
                            <tr className="border-b border-border">
                                <th className={TH}>When</th>
                                <th className={TH}>Admin</th>
                                <th className={TH}>Mailbox</th>
                                <th className={TH}>Action</th>
                                <th className={TH}>Reason</th>
                            </tr>
                        </thead>
                        <tbody>
                            {rows.map((a) => (
                                <tr key={a.id} className={TR}>
                                    <td className={cn(TD, "whitespace-nowrap tabular-nums text-muted-foreground")}>
                                        {new Date(a.created_at).toLocaleString()}
                                    </td>
                                    <td className={TD}>{a.admin_email || a.admin_user_id.slice(0, 8)}</td>
                                    <td className={cn(TD, "font-medium text-foreground")}>{a.email || a.email_account_id.slice(0, 8)}</td>
                                    <td className={TD}>
                                        <StatusBadge tone={ACTION_TONE[a.action] ?? "neutral"} dot>
                                            {a.action}
                                        </StatusBadge>
                                    </td>
                                    <td className={cn(TD, "max-w-md")}>
                                        {a.reason ? (
                                            <span className="block truncate" title={a.reason}>
                                                {a.reason}
                                            </span>
                                        ) : (
                                            <span className="text-subtle-foreground">None</span>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </TableFrame>
            )}
        </div>
    );
}

const ACTION_TONE: Record<string, Tone> = {
    block: "danger",
    unblock: "success",
};
