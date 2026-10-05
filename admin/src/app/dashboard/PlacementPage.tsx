// Inbox placement seed panel. The instance panel is the set of mailboxes every
// workspace's placement tests send to; this page is where the operator builds
// it and watches the tests that run against it. No polling: the realtime
// spine's placement group invalidates ["admin","placement"] on
// PLACEMENT_TEST_UPDATED.

import { useMemo, useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { toast } from "sonner";
import { Play, Plus, Search, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, Panel, Section, StatusBadge } from "@/components/ui/kit";
import { ErrorState } from "@/components/ErrorState";
import { useConfirm } from "@/components/ConfirmDialog";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import { TONE, TONE_TEXT, type Tone } from "@/lib/tones";
import { cn } from "@/lib/utils";
import {
    listPlacementSeeds,
    listPlacementTests,
    searchPlacementSeedCandidates,
    setPlacementSeed,
    type AdminPlacementSeed,
    type PlacementTestView,
} from "@/lib/api/client/admin/placement";
import { absolute, relative } from "@/app/dashboard/jobs/format";
import { TestStatusBadge } from "@/app/dashboard/placement/badges";
import { describeError, ORIGIN_LABEL, PANEL_LABEL, pct } from "@/app/dashboard/placement/format";
import { RunTestDialog } from "@/app/dashboard/placement/RunTestDialog";
import { TestDetailSheet } from "@/app/dashboard/placement/TestDetailSheet";
import { useDebounced } from "@/app/dashboard/placement/useDebounced";

// A provider family with fewer seeds than this gives a verdict one mailbox can swing.
const MIN_SEEDS_PER_FAMILY = 3;

const SEED_STATUS_TONE: Record<string, Tone> = {
    active: "success",
    inactive: "warning",
    revoked: "danger",
};

function SeedStatusBadge({ status }: { status: string }) {
    return (
        <StatusBadge tone={SEED_STATUS_TONE[status] ?? "neutral"} dot>
            {status}
        </StatusBadge>
    );
}

function IdLink({ to, id, canView }: { to: string; id: string; canView: boolean }) {
    if (!canView) return <span className="font-mono text-xs text-muted-foreground">{id.slice(0, 8)}</span>;
    return (
        <Link
            to={to}
            onClick={(e) => e.stopPropagation()}
            className="font-mono text-xs text-[var(--admin-accent-strong)] hover:underline"
        >
            {id.slice(0, 8)}
        </Link>
    );
}

export default function PlacementPage() {
    const [params, setParams] = useSearchParams();
    const openTest = params.get("test");
    const [runOpen, setRunOpen] = useState(false);
    const canManage = useAdminPerm(AdminPerm.ManageWarmupBans);

    function setOpenTest(id: string | null) {
        setParams(
            (p) => {
                if (id) p.set("test", id);
                else p.delete("test");
                return p;
            },
            { replace: true },
        );
    }

    return (
        <div>
            <PageHeader
                title="Seed panel"
                description="The mailboxes every workspace's inbox placement tests send to on this instance, and every test run against a panel."
            >
                {canManage && (
                    <Button size="sm" onClick={() => setRunOpen(true)}>
                        <Play /> Run a test
                    </Button>
                )}
            </PageHeader>

            <SeedsSection canManage={canManage} />
            {canManage && <AddSeedsSection />}
            <TestsSection onOpen={setOpenTest} />

            <TestDetailSheet
                testId={openTest}
                onOpenChange={(open) => !open && setOpenTest(null)}
                onOpenTest={setOpenTest}
            />
            {canManage && (
                <RunTestDialog
                    open={runOpen}
                    onOpenChange={setRunOpen}
                    onStarted={(tests) => {
                        setRunOpen(false);
                        if (tests[0]) setOpenTest(tests[0].id);
                    }}
                />
            )}
        </div>
    );
}

function SeedsSection({ canManage }: { canManage: boolean }) {
    const qc = useQueryClient();
    const confirm = useConfirm();
    const canViewOrgs = useAdminPerm(AdminPerm.ViewOrganizations);
    const canViewWorkers = useAdminPerm(AdminPerm.ViewWorkers);

    const seedsQ = useQuery({
        queryKey: ["admin", "placement", "seeds"],
        queryFn: listPlacementSeeds,
    });
    const seeds = useMemo(() => seedsQ.data ?? [], [seedsQ.data]);

    const mix = useMemo(() => {
        const counts = new Map<string, number>();
        for (const s of seeds) {
            const label = s.family_label || "Other provider";
            counts.set(label, (counts.get(label) ?? 0) + 1);
        }
        return [...counts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
    }, [seeds]);

    const remove = useMutation({
        mutationFn: (seed: AdminPlacementSeed) => setPlacementSeed(seed.id, false),
        onSuccess: (seed) => {
            toast.success(`${seed.email} removed from the panel`);
            qc.invalidateQueries({ queryKey: ["admin", "placement"] });
        },
        onError: (err) => toast.error(describeError(err).message || "Could not remove the seed"),
    });

    async function onRemove(seed: AdminPlacementSeed) {
        const ok = await confirm({
            title: "Remove this seed?",
            description: `${seed.email} stops receiving copies from new placement tests. The mailbox stays connected, and its warmup stays off until someone turns it back on.`,
            confirmLabel: "Remove seed",
            destructive: true,
        });
        if (!ok) return;
        remove.mutate(seed);
    }

    const columns: Column<AdminPlacementSeed>[] = [
        {
            id: "mailbox",
            header: "Mailbox",
            cell: (s) => (
                <div className="min-w-0 py-1.5">
                    <div className="truncate font-mono text-[12.5px] text-foreground">{s.email}</div>
                    {s.name && <div className="truncate text-xs text-muted-foreground">{s.name}</div>}
                </div>
            ),
            csv: (s) => s.email,
        },
        {
            id: "family",
            header: "Provider",
            cell: (s) => <span className="whitespace-nowrap">{s.family_label}</span>,
            csv: (s) => s.family_label,
        },
        {
            id: "status",
            header: "Status",
            cell: (s) => <SeedStatusBadge status={s.status} />,
            csv: (s) => s.status,
        },
        {
            id: "worker",
            header: "Worker",
            cell: (s) =>
                s.worker_id ? (
                    <IdLink to={`/workers/${s.worker_id}`} id={s.worker_id} canView={canViewWorkers} />
                ) : (
                    <StatusBadge tone="warning" title="Nothing syncs this mailbox, so copies sent to it read as missing">
                        unassigned
                    </StatusBadge>
                ),
            csv: (s) => s.worker_id ?? "",
        },
        {
            id: "organization",
            header: "Organization",
            cell: (s) =>
                s.organization_id ? (
                    <IdLink
                        to={`/organizations/${s.organization_id}`}
                        id={s.organization_id}
                        canView={canViewOrgs}
                    />
                ) : (
                    <span className="text-subtle-foreground">—</span>
                ),
            csv: (s) => s.organization_id ?? "",
        },
    ];
    if (canManage) {
        columns.push({
            id: "actions",
            header: "",
            align: "right",
            cell: (s) => (
                <Button
                    size="xs"
                    variant="ghost"
                    className="text-muted-foreground hover:text-foreground"
                    disabled={remove.isPending && remove.variables?.id === s.id}
                    onClick={(e) => {
                        e.stopPropagation();
                        void onRemove(s);
                    }}
                >
                    <Trash2 /> Remove
                </Button>
            ),
        });
    }

    return (
        <Section
            className="mt-0"
            title="Seeds"
            description="Copies are spread round-robin across provider families, so every family on the panel is covered by each test. Seeds on the sender's own domain are always skipped."
        >
            <Panel
                title="Mix by provider"
                actions={
                    seeds.length > 0 ? (
                        <span className="text-xs tabular-nums text-muted-foreground">
                            {seeds.length.toLocaleString()} {seeds.length === 1 ? "seed" : "seeds"}
                        </span>
                    ) : undefined
                }
                className="mb-4"
                bodyClassName="space-y-3"
            >
                {seedsQ.isLoading ? (
                    <Skeleton className="h-6 w-2/3" />
                ) : mix.length === 0 ? (
                    <p className="text-[13px] text-muted-foreground">
                        The panel is empty, so no workspace can test on it yet.
                    </p>
                ) : (
                    <div className="flex flex-wrap gap-1.5">
                        {mix.map(([label, n]) => {
                            const thin = n < MIN_SEEDS_PER_FAMILY;
                            return (
                                <span
                                    key={label}
                                    className={cn(
                                        "inline-flex h-6 items-center gap-1.5 rounded-md border px-2 text-xs",
                                        thin ? TONE.warning : "border-border bg-muted/40 text-foreground",
                                    )}
                                    title={thin ? `Fewer than ${MIN_SEEDS_PER_FAMILY} seeds` : undefined}
                                >
                                    {label}
                                    <span className={cn("font-semibold tabular-nums", !thin && "text-muted-foreground")}>
                                        {n}
                                    </span>
                                </span>
                            );
                        })}
                    </div>
                )}
                <p className="max-w-3xl text-xs leading-relaxed text-muted-foreground">
                    A healthy panel is weighted toward Microsoft 365 and Google Workspace, where most B2B recipients
                    are, with a few Gmail, Outlook.com and Yahoo seeds, and at least {MIN_SEEDS_PER_FAMILY} per
                    provider. Seeds must be mailboxes nobody reads or engages with: opening, filing or replying to a
                    copy teaches the provider to trust the sender and skews every later result.
                </p>
            </Panel>

            <DataTable
                columns={columns}
                rows={seeds}
                getRowId={(s) => s.id}
                loading={seedsQ.isLoading}
                error={seedsQ.error}
                onRetry={() => seedsQ.refetch()}
                errorTitle="Failed to load the seed panel"
                storageKey="admin.placement.seeds"
                csvName="warmbly-placement-seeds"
                noun="seeds"
                emptyTitle="No seeds yet"
                emptyHint={
                    canManage
                        ? "Add connected mailboxes below to build the panel."
                        : "An admin with the manage warmup bans permission can add them."
                }
            />
        </Section>
    );
}

function AddSeedsSection() {
    const qc = useQueryClient();
    const confirm = useConfirm();
    const [search, setSearch] = useState("");
    const debounced = useDebounced(search.trim(), 250);

    const candidatesQ = useQuery({
        queryKey: ["admin", "placement", "candidates", debounced],
        queryFn: () => searchPlacementSeedCandidates(debounced),
        enabled: debounced.length >= 2,
        staleTime: 30_000,
    });
    const rows = candidatesQ.data ?? [];

    const add = useMutation({
        mutationFn: (seed: AdminPlacementSeed) => setPlacementSeed(seed.id, true),
        onSuccess: (seed) => {
            toast.success(`${seed.email} added to the panel, with its warmup turned off`);
            qc.invalidateQueries({ queryKey: ["admin", "placement"] });
        },
        onError: (err) => toast.error(describeError(err).message || "Could not add the seed"),
    });

    async function onAdd(seed: AdminPlacementSeed) {
        if (seed.seed_scope === "workspace") {
            const ok = await confirm({
                title: "Move this workspace seed?",
                description: `${seed.email} is one of its workspace's own test inboxes. Adding it here moves it to the instance panel, so its workspace can no longer test on it privately.`,
                confirmLabel: "Add to the panel",
            });
            if (!ok) return;
        }
        add.mutate(seed);
    }

    return (
        <Section
            title="Add seeds"
            description="Any connected mailbox can join the panel. Adding one turns its warmup off and keeps it out of campaign sending, so pick mailboxes that exist only to receive tests."
        >
            <div className="relative mb-3 max-w-md">
                <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-subtle-foreground" />
                <Input
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search connected mailboxes by address…"
                    className="h-8 pl-8"
                    autoComplete="off"
                />
            </div>

            {debounced.length < 2 ? null : candidatesQ.isLoading ? (
                <Skeleton className="h-24 w-full rounded-lg" />
            ) : candidatesQ.error ? (
                <ErrorState error={candidatesQ.error} title="Search failed" onRetry={() => candidatesQ.refetch()} />
            ) : rows.length === 0 ? (
                <div className="surface-lit rounded-xl border border-border bg-card">
                    <EmptyState icon={Search} title="No connected mailbox matches." className="py-10" />
                </div>
            ) : (
                <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
                    <div className="overflow-x-auto">
                        <table className="w-full border-collapse text-[13px]">
                            <thead>
                                <tr className="border-b border-border">
                                    <th className={TH}>Mailbox</th>
                                    <th className={TH}>Provider</th>
                                    <th className={TH}>Status</th>
                                    <th className={cn(TH, "text-right")}>
                                        <span className="sr-only">Actions</span>
                                    </th>
                                </tr>
                            </thead>
                            <tbody>
                                {rows.map((m) => {
                                    const onPanel = m.seed_scope === "instance";
                                    return (
                                        <tr
                                            key={m.id}
                                            className="h-10 border-b border-border/70 transition-colors last:border-0 hover:bg-accent/50"
                                        >
                                            <td className={TD}>
                                                <div className="flex items-center gap-2">
                                                    <span className="font-mono text-[12.5px]">{m.email}</span>
                                                    {m.seed_scope === "workspace" && (
                                                        <StatusBadge tone="info">workspace seed</StatusBadge>
                                                    )}
                                                </div>
                                            </td>
                                            <td className={cn(TD, "whitespace-nowrap")}>{m.family_label}</td>
                                            <td className={TD}>
                                                <SeedStatusBadge status={m.status} />
                                            </td>
                                            <td className={cn(TD, "text-right")}>
                                                <Button
                                                    size="xs"
                                                    variant={onPanel ? "ghost" : "outline"}
                                                    disabled={onPanel || (add.isPending && add.variables?.id === m.id)}
                                                    onClick={() => void onAdd(m)}
                                                >
                                                    <Plus /> {onPanel ? "On the panel" : "Add"}
                                                </Button>
                                            </td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}
        </Section>
    );
}

const TH = "h-9 whitespace-nowrap px-3 text-left text-xs font-medium text-muted-foreground first:pl-4 last:pr-4";
const TD = "px-3 first:pl-4 last:pr-4";

function TestsSection({ onOpen }: { onOpen: (id: string) => void }) {
    const testsQ = useInfiniteQuery({
        queryKey: ["admin", "placement", "tests"],
        queryFn: ({ pageParam }) => listPlacementTests(pageParam),
        initialPageParam: undefined as string | undefined,
        getNextPageParam: (last) =>
            last.pagination?.has_more ? (last.pagination.next_cursor ?? undefined) : undefined,
    });
    const rows = useMemo(() => (testsQ.data?.pages ?? []).flatMap((p) => p.data ?? []), [testsQ.data]);
    const total = testsQ.data?.pages[0]?.pagination?.total;

    const columns: Column<PlacementTestView>[] = [
        {
            id: "created",
            header: "Created",
            cell: (t) => (
                <span className="whitespace-nowrap text-muted-foreground" title={absolute(t.created_at)}>
                    {relative(t.created_at)}
                </span>
            ),
            csv: (t) => t.created_at,
        },
        {
            id: "sender",
            header: "Sender",
            cell: (t) => <span className="font-mono text-[12.5px]">{t.sender_email || "—"}</span>,
            csv: (t) => t.sender_email,
        },
        {
            id: "subject",
            header: "Subject",
            cell: (t) => <span className="block max-w-72 truncate">{t.subject || "—"}</span>,
            csv: (t) => t.subject,
        },
        {
            id: "panel",
            header: "Panel",
            cell: (t) => <span className="whitespace-nowrap text-muted-foreground">{PANEL_LABEL[t.panel] ?? t.panel}</span>,
            csv: (t) => t.panel,
        },
        {
            id: "origin",
            header: "Origin",
            cell: (t) => <span className="whitespace-nowrap text-muted-foreground">{ORIGIN_LABEL[t.origin] ?? t.origin}</span>,
            csv: (t) => t.origin,
        },
        {
            id: "status",
            header: "Status",
            cell: (t) => <TestStatusBadge status={t.status} />,
            csv: (t) => t.status,
        },
        {
            id: "inbox",
            header: "Inbox",
            align: "right",
            cell: (t) => <span className="font-medium tabular-nums">{pct(t.summary?.inbox_rate)}</span>,
            csv: (t) => pct(t.summary?.inbox_rate),
        },
        {
            id: "spam",
            header: "Spam",
            align: "right",
            cell: (t) => (
                <span className={cn("tabular-nums", (t.summary?.spam ?? 0) > 0 ? TONE_TEXT.danger : "text-muted-foreground")}>
                    {pct(t.summary?.spam_rate)}
                </span>
            ),
            csv: (t) => pct(t.summary?.spam_rate),
        },
        {
            id: "delivered",
            header: "Delivered",
            align: "right",
            cell: (t) => (
                <span className="whitespace-nowrap tabular-nums">
                    {t.summary?.delivered ?? 0}
                    <span className="text-muted-foreground"> / {t.summary?.total ?? 0}</span>
                </span>
            ),
            csv: (t) => t.summary?.delivered ?? 0,
        },
    ];

    return (
        <Section
            title="Recent tests"
            description={`Every placement test on this instance, from every workspace, newest first.${
                total !== undefined ? ` ${total.toLocaleString()} in total.` : ""
            }`}
        >
            <DataTable
                columns={columns}
                rows={rows}
                getRowId={(t) => t.id}
                loading={testsQ.isLoading}
                error={testsQ.error}
                onRetry={() => testsQ.refetch()}
                onRowClick={(t) => onOpen(t.id)}
                errorTitle="Failed to load placement tests"
                storageKey="admin.placement.tests"
                csvName="warmbly-placement-tests"
                noun="tests"
                emptyTitle="No placement tests yet"
                emptyHint="Tests appear here once a workspace, a campaign monitor or an admin runs one."
            />
            {testsQ.hasNextPage && (
                <div className="mt-3 flex justify-center">
                    <Button
                        size="sm"
                        variant="outline"
                        onClick={() => testsQ.fetchNextPage()}
                        disabled={testsQ.isFetchingNextPage}
                    >
                        {testsQ.isFetchingNextPage ? "Loading…" : "Load more"}
                    </Button>
                </div>
            )}
        </Section>
    );
}
