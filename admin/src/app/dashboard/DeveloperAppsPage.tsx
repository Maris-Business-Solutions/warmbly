// Developer apps: every OAuth app on the instance and who may build them.
// Apps: suspend (no sign-ins, tokens stop working, out of the directory),
// revoke every token, remove a logo, block the workspace or the person who
// registered it. Directory: feature, hide or restore published listings.
// Blocked: who cannot register or publish apps, and lifting it.

import { useEffect, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useSearchParams } from "react-router-dom";
import { toast } from "sonner";
import { MoreHorizontal } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Explorer, FilterGroup, SearchFilter, SelectFilter } from "@/components/data/Explorer";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useConfirm } from "@/components/ConfirmDialog";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import { useCursorPager } from "@/lib/useCursorPager";
import {
    createDeveloperBlock,
    deleteDeveloperBlock,
    listDeveloperBlocks,
    listOAuthApps,
    removeOAuthAppLogo,
    revokeOAuthAppGrants,
    suspendOAuthApp,
    unsuspendOAuthApp,
} from "@/lib/api/client/admin/oauthApps";
import type { AdminOAuthApp } from "@/lib/api/models/admin";
import AppListingsPage from "./AppListingsPage";

type Tab = "apps" | "directory" | "blocked";

export default function DeveloperAppsPage() {
    const [params, setParams] = useSearchParams();
    const tab = (["apps", "directory", "blocked"].includes(params.get("tab") ?? "") ? params.get("tab") : "apps") as Tab;
    return (
        <div>
            <PageHeader
                title="Developer apps"
                description="OAuth apps registered on this instance, their directory listings, and who may build them."
            />
            <Tabs value={tab} onValueChange={(v) => setParams(v === "apps" ? {} : { tab: v }, { replace: true })}>
                <TabsList variant="line">
                    <TabsTrigger value="apps">Apps</TabsTrigger>
                    <TabsTrigger value="directory">Directory</TabsTrigger>
                    <TabsTrigger value="blocked">Blocked</TabsTrigger>
                </TabsList>
                <TabsContent value="apps" className="mt-5">
                    <AppsTab />
                </TabsContent>
                <TabsContent value="directory" className="mt-5">
                    <AppListingsPage embedded />
                </TabsContent>
                <TabsContent value="blocked" className="mt-5">
                    <BlockedTab />
                </TabsContent>
            </Tabs>
        </div>
    );
}

const STATUS_OPTIONS = [
    { value: "any", label: "Any status" },
    { value: "active", label: "Active" },
    { value: "disabled", label: "Disabled by owner" },
    { value: "suspended", label: "Suspended" },
];

type Action =
    | { kind: "suspend"; app: AdminOAuthApp }
    | { kind: "block_org"; app: AdminOAuthApp }
    | { kind: "block_creator"; app: AdminOAuthApp };

function AppsTab() {
    const canManage = useAdminPerm(AdminPerm.ManageOrganizations);
    const qc = useQueryClient();
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState("any");
    const pager = useCursorPager();
    const { reset } = pager;
    const [action, setAction] = useState<Action | null>(null);
    const confirm = useConfirm();

    const filterKey = JSON.stringify({ query, status });
    useEffect(() => {
        reset();
    }, [filterKey, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "oauth-apps", filterKey, pager.cursor],
        queryFn: () =>
            listOAuthApps({
                q: query.trim() || undefined,
                status: status === "any" ? "" : (status as "active"),
                limit: 50,
                cursor: pager.cursor,
            }),
        staleTime: 30_000,
        placeholderData: keepPreviousData,
    });
    const rows = data?.data ?? [];

    const refresh = () => {
        qc.invalidateQueries({ queryKey: ["admin", "oauth-apps"] });
        qc.invalidateQueries({ queryKey: ["admin", "oauth-blocks"] });
        qc.invalidateQueries({ queryKey: ["admin", "app-listings"] });
    };
    const run = useMutation({
        mutationFn: async (fn: () => Promise<unknown>) => fn(),
        onSuccess: refresh,
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    const columns: Column<AdminOAuthApp>[] = [
        {
            id: "app",
            header: "App",
            cell: (r) => (
                <div className="flex items-center gap-2 min-w-0">
                    {r.logo_url ? (
                        <img src={r.logo_url} alt="" className="size-7 rounded border object-cover shrink-0" />
                    ) : (
                        <div className="size-7 rounded border bg-muted flex items-center justify-center text-xs font-semibold shrink-0">
                            {(r.name[0] ?? "?").toUpperCase()}
                        </div>
                    )}
                    <div className="min-w-0">
                        <div className="font-medium truncate">{r.name}</div>
                        <div className="font-mono text-[10px] text-muted-foreground truncate">{r.client_id}</div>
                    </div>
                </div>
            ),
            csv: (r) => r.name,
        },
        {
            id: "workspace",
            header: "Workspace",
            cell: (r) => (
                <div className="min-w-0">
                    <Link
                        to={`/organizations/${r.organization_id}`}
                        onClick={(e) => e.stopPropagation()}
                        className="text-xs font-medium text-[var(--admin-accent-strong)] hover:underline"
                    >
                        {r.organization_name || r.organization_id}
                    </Link>
                    <div className="text-[10px] text-muted-foreground truncate">{r.created_by_email}</div>
                </div>
            ),
            csv: (r) => r.organization_name,
        },
        {
            id: "permissions",
            header: "Permissions",
            cell: (r) => {
                const writes = r.permissions.filter((p) => p.category !== "read").length;
                return (
                    <span className="text-xs" title={r.permissions.map((p) => p.name.toLowerCase()).join(", ")}>
                        {r.permissions.length}
                        {writes > 0 && <span className="text-amber-700"> ({writes} write)</span>}
                    </span>
                );
            },
            csv: (r) => r.permissions.map((p) => p.name).join(" "),
        },
        {
            id: "installs",
            header: "Installs",
            align: "right",
            cell: (r) => <span className="tabular-nums text-xs">{r.installs.toLocaleString()}</span>,
            csv: (r) => r.installs,
        },
        {
            id: "listing",
            header: "Directory",
            cell: (r) =>
                r.listing_status ? (
                    <span className="text-xs">{r.listing_status === "published" ? "Link only" : r.listing_status === "featured" ? "Featured" : "Hidden"}</span>
                ) : (
                    <span className="text-xs text-muted-foreground">Not published</span>
                ),
            csv: (r) => r.listing_status ?? "",
        },
        {
            id: "status",
            header: "Status",
            cell: (r) => (
                <div className="space-y-1">
                    {r.suspended_at ? (
                        <Badge variant="outline" className="text-[10px] border-red-300 text-red-700 bg-red-50">suspended</Badge>
                    ) : r.status === "disabled" ? (
                        <Badge variant="outline" className="text-[10px]">disabled by owner</Badge>
                    ) : (
                        <Badge variant="outline" className="text-[10px] border-emerald-300 text-emerald-700 bg-emerald-50">active</Badge>
                    )}
                    {r.suspended_reason && (
                        <div className="text-[10px] text-muted-foreground max-w-xs truncate" title={r.suspended_reason}>
                            "{r.suspended_reason}"
                        </div>
                    )}
                    {(r.org_blocked || r.creator_blocked) && (
                        <div className="text-[10px] text-amber-700">{r.org_blocked ? "workspace blocked" : "creator blocked"}</div>
                    )}
                </div>
            ),
            csv: (r) => (r.suspended_at ? "suspended" : r.status),
        },
        {
            id: "created",
            header: "Created",
            cell: (r) => <span className="text-xs text-muted-foreground">{new Date(r.created_at).toLocaleDateString()}</span>,
            csv: (r) => r.created_at,
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (r) => (
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button size="sm" variant="ghost" disabled={!canManage} onClick={(e) => e.stopPropagation()} aria-label="Actions">
                            <MoreHorizontal className="size-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="min-w-48" onClick={(e) => e.stopPropagation()}>
                        {r.suspended_at ? (
                            <DropdownMenuItem
                                onSelect={() => run.mutate(() => unsuspendOAuthApp(r.id).then(() => toast.success(`${r.name} unsuspended`)))}
                            >
                                Unsuspend
                            </DropdownMenuItem>
                        ) : (
                            <DropdownMenuItem onSelect={() => setAction({ kind: "suspend", app: r })} className="text-red-600">
                                Suspend…
                            </DropdownMenuItem>
                        )}
                        <DropdownMenuItem
                            disabled={r.installs === 0}
                            onSelect={async () => {
                                const ok = await confirm({
                                    title: `Revoke every token for ${r.name}?`,
                                    description: `Every workspace that installed it (${r.installs}) has to authorize it again. This cannot be undone.`,
                                    confirmLabel: "Revoke every token",
                                    destructive: true,
                                });
                                if (ok) {
                                    run.mutate(() =>
                                        revokeOAuthAppGrants(r.id).then((res) => toast.success(`${res.revoked} tokens revoked`)),
                                    );
                                }
                            }}
                        >
                            Revoke every token
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            disabled={!r.logo_url}
                            onSelect={() => run.mutate(() => removeOAuthAppLogo(r.id).then(() => toast.success("Logo removed")))}
                        >
                            Remove logo
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem disabled={r.org_blocked} onSelect={() => setAction({ kind: "block_org", app: r })}>
                            Block workspace from building apps…
                        </DropdownMenuItem>
                        <DropdownMenuItem
                            disabled={r.creator_blocked || !r.created_by}
                            onSelect={() => setAction({ kind: "block_creator", app: r })}
                        >
                            Block {r.created_by_email || "creator"}…
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            ),
        },
    ];

    return (
        <>
            <Explorer
                activeCount={(query ? 1 : 0) + (status !== "any" ? 1 : 0)}
                onReset={() => {
                    setQuery("");
                    setStatus("any");
                }}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="App, client ID, workspace, email…" />
                        </FilterGroup>
                        <FilterGroup label="Status">
                            <SelectFilter value={status} onChange={setStatus} options={STATUS_OPTIONS} placeholder="Any status" />
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
                    errorTitle="Failed to load apps"
                    storageKey="admin.oauth-apps"
                    csvName="warmbly-oauth-apps"
                    noun="apps"
                    emptyTitle="No apps"
                    emptyHint="No apps match these filters."
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
            {action && <ActionDialog action={action} onDone={refresh} onOpenChange={(v) => !v && setAction(null)} />}
        </>
    );
}

function ActionDialog({ action, onDone, onOpenChange }: { action: Action; onDone: () => void; onOpenChange: (v: boolean) => void }) {
    const [reason, setReason] = useState("");
    const [suspendApps, setSuspendApps] = useState(true);
    const app = action.app;
    const copy =
        action.kind === "suspend"
            ? {
                  title: `Suspend ${app.name}`,
                  body: "No one can sign in with it, its tokens stop working, its webhooks stop and it leaves the directory. The owner cannot lift a suspension; you can.",
                  cta: "Suspend",
              }
            : action.kind === "block_org"
              ? {
                    title: `Block ${app.organization_name || "this workspace"}`,
                    body: "The workspace can no longer register or publish OAuth apps. Its existing apps keep working unless you suspend them too.",
                    cta: "Block workspace",
                }
              : {
                    title: `Block ${app.created_by_email || "this person"}`,
                    body: "They can no longer register or publish OAuth apps in any workspace. Apps they already registered keep working unless you suspend them too.",
                    cta: "Block person",
                };

    const mutation = useMutation({
        mutationFn: async (): Promise<{ suspended_apps?: number }> => {
            if (action.kind === "suspend") {
                await suspendOAuthApp(app.id, reason);
                return {};
            }
            return createDeveloperBlock({
                organization_id: action.kind === "block_org" ? app.organization_id : undefined,
                user_id: action.kind === "block_creator" ? app.created_by : undefined,
                reason,
                suspend_apps: suspendApps,
            });
        },
        onSuccess: (res) => {
            const suspended = res.suspended_apps;
            toast.success(action.kind === "suspend" ? `${app.name} suspended` : `Blocked${suspended ? `, ${suspended} apps suspended` : ""}`);
            onDone();
            onOpenChange(false);
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    return (
        <Dialog open onOpenChange={onOpenChange}>
            <DialogContent className="max-w-lg">
                <DialogHeader>
                    <DialogTitle>{copy.title}</DialogTitle>
                    <DialogDescription>{copy.body}</DialogDescription>
                </DialogHeader>
                <div className="space-y-3">
                    <div>
                        <Label htmlFor="moderation-reason" className="text-xs font-medium">
                            Reason (required, shown to the developer)
                        </Label>
                        <Textarea
                            id="moderation-reason"
                            rows={3}
                            maxLength={1000}
                            value={reason}
                            onChange={(e) => setReason(e.target.value)}
                            placeholder="What was wrong, so they can fix it or appeal"
                        />
                    </div>
                    {action.kind !== "suspend" && (
                        <label className="flex items-center gap-2 text-xs">
                            <Checkbox checked={suspendApps} onCheckedChange={(v) => setSuspendApps(v === true)} />
                            Also suspend the apps they already have
                        </label>
                    )}
                </div>
                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => {
                            if (!reason.trim()) {
                                toast.error("Add a reason; the developer sees it");
                                return;
                            }
                            mutation.mutate();
                        }}
                        disabled={mutation.isPending}
                        className="bg-red-600 hover:bg-red-700 text-white"
                    >
                        {mutation.isPending ? "Working…" : copy.cta}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

function BlockedTab() {
    const canManage = useAdminPerm(AdminPerm.ManageOrganizations);
    const qc = useQueryClient();
    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "oauth-blocks"],
        queryFn: listDeveloperBlocks,
        staleTime: 30_000,
    });
    const unblock = useMutation({
        mutationFn: (id: string) => deleteDeveloperBlock(id),
        onSuccess: () => {
            toast.success("Unblocked. Apps suspended with the block stay suspended until you unsuspend them.");
            qc.invalidateQueries({ queryKey: ["admin", "oauth-blocks"] });
            qc.invalidateQueries({ queryKey: ["admin", "oauth-apps"] });
        },
        onError: (err: Error) => toast.error(err.message || "Unblock failed"),
    });
    const rows = data?.data ?? [];

    const columns: Column<(typeof rows)[number]>[] = [
        {
            id: "who",
            header: "Blocked",
            cell: (r) =>
                r.organization_id ? (
                    <Link to={`/organizations/${r.organization_id}`} className="text-xs font-medium text-[var(--admin-accent-strong)] hover:underline">
                        {r.organization_name || r.organization_id}
                        <span className="ml-1.5 text-[10px] text-muted-foreground">workspace</span>
                    </Link>
                ) : (
                    <Link to={`/users/${r.user_id}`} className="text-xs font-medium text-[var(--admin-accent-strong)] hover:underline">
                        {r.user_email || r.user_id}
                        <span className="ml-1.5 text-[10px] text-muted-foreground">person</span>
                    </Link>
                ),
            csv: (r) => r.organization_name || r.user_email || "",
        },
        {
            id: "reason",
            header: "Reason",
            cell: (r) => (
                <span className="text-xs max-w-md truncate block" title={r.reason}>
                    {r.reason}
                </span>
            ),
            csv: (r) => r.reason,
        },
        {
            id: "by",
            header: "By",
            cell: (r) => <span className="text-xs text-muted-foreground">{r.blocked_by_email || "unknown"}</span>,
            csv: (r) => r.blocked_by_email ?? "",
        },
        {
            id: "when",
            header: "Since",
            cell: (r) => <span className="text-xs text-muted-foreground">{new Date(r.created_at).toLocaleDateString()}</span>,
            csv: (r) => r.created_at,
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (r) => (
                <Button size="sm" variant="outline" className="text-xs" disabled={!canManage || unblock.isPending} onClick={() => unblock.mutate(r.id)}>
                    Unblock
                </Button>
            ),
        },
    ];

    return (
        <DataTable
            columns={columns}
            rows={rows}
            getRowId={(r) => r.id}
            loading={isLoading}
            error={error}
            onRetry={() => refetch()}
            errorTitle="Failed to load blocks"
            storageKey="admin.oauth-blocks"
            csvName="warmbly-developer-blocks"
            noun="blocks"
            emptyTitle="Nobody is blocked"
            emptyHint="Block a workspace or a person from an app's menu on the Apps tab."
        />
    );
}
