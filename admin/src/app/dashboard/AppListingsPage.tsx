// Community app directory. A published app is link only until it is featured
// here or enough workspaces use it; featuring lists it in every workspace's
// Integrations page, hiding takes it down, link included. An edit to a featured
// listing, or to its app, removes the feature.

import { useEffect, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { EyeOff, ExternalLink, RotateCcw, Star, StarOff } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Explorer, FilterGroup, SearchFilter, SelectFilter } from "@/components/data/Explorer";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import { useCursorPager } from "@/lib/useCursorPager";
import { listAppListings, setAppListingStatus } from "@/lib/api/client/admin/appListings";
import type { AdminAppListing, AppListingStatus } from "@/lib/api/models/admin";

const STATUS_LABEL: Record<AppListingStatus, string> = {
    published: "Link only",
    featured: "Featured",
    hidden: "Hidden",
};

const STATUS_TONE: Record<AppListingStatus, string> = {
    published: "border-zinc-300 text-zinc-600 bg-zinc-50",
    featured: "border-sky-300 text-sky-700 bg-sky-50",
    hidden: "border-red-300 text-red-700 bg-red-50",
};

const STATUS_OPTIONS = [
    { value: "any", label: "Any status" },
    { value: "published", label: "Link only" },
    { value: "featured", label: "Featured" },
    { value: "hidden", label: "Hidden" },
];

function hostOf(url: string): string {
    try {
        return new URL(url).host;
    } catch {
        return url;
    }
}

export default function AppListingsPage({ embedded = false }: { embedded?: boolean }) {
    const canManage = useAdminPerm(AdminPerm.ManageOrganizations);
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState<AppListingStatus | "any">("any");
    const pager = useCursorPager();
    const { reset } = pager;
    const [acting, setActing] = useState<{ item: AdminAppListing; to: AppListingStatus } | null>(null);

    const filterKey = JSON.stringify({ query, status });
    useEffect(() => {
        reset();
    }, [filterKey, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "app-listings", filterKey, pager.cursor],
        queryFn: () =>
            listAppListings({
                q: query.trim() || undefined,
                status: status === "any" ? "" : status,
                limit: 50,
                cursor: pager.cursor,
            }),
        staleTime: 30_000,
        placeholderData: keepPreviousData,
    });
    const rows = data?.data ?? [];

    const columns: Column<AdminAppListing>[] = [
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
                        <div className="font-mono text-[10px] text-muted-foreground truncate">{r.slug}</div>
                    </div>
                </div>
            ),
            csv: (r) => r.name,
        },
        {
            id: "workspace",
            header: "Publisher",
            cell: (r) => (
                <Link
                    to={`/organizations/${r.organization_id}`}
                    onClick={(e) => e.stopPropagation()}
                    className="text-xs font-medium text-[var(--admin-accent-strong)] hover:underline"
                >
                    {r.organization_name || r.organization_id}
                </Link>
            ),
            csv: (r) => r.organization_name,
        },
        {
            id: "tagline",
            header: "Listing",
            cell: (r) => (
                <div className="max-w-sm">
                    <div className="text-xs truncate" title={r.tagline}>
                        {r.tagline}
                    </div>
                    <div className="text-[10px] text-muted-foreground capitalize">{r.category}</div>
                </div>
            ),
            csv: (r) => r.tagline,
        },
        {
            id: "install",
            header: "Install URL",
            cell: (r) => (
                <a
                    href={r.install_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    title={r.install_url}
                    className="text-xs font-mono inline-flex items-center gap-1 hover:underline"
                >
                    {hostOf(r.install_url)}
                    <ExternalLink className="size-3" />
                </a>
            ),
            csv: (r) => r.install_url,
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
            id: "status",
            header: "Status",
            cell: (r) => (
                <div>
                    <Badge variant="outline" className={`text-[10px] ${STATUS_TONE[r.status]}`}>
                        {STATUS_LABEL[r.status]}
                    </Badge>
                    {r.status === "published" && r.listed && (
                        <div className="text-[10px] text-muted-foreground mt-1">listed by installs</div>
                    )}
                    {r.app_status !== "active" && <div className="text-[10px] text-muted-foreground mt-1">app {r.app_status}</div>}
                    {r.status === "hidden" && r.status_note && (
                        <div className="text-[10px] text-muted-foreground mt-1 max-w-xs truncate" title={r.status_note}>
                            "{r.status_note}"
                        </div>
                    )}
                </div>
            ),
            csv: (r) => r.status,
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (r) => (
                <div className="space-x-1.5 whitespace-nowrap">
                    {r.status === "featured" ? (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "published" })}>
                            <StarOff className="size-3" /> Unfeature
                        </ActionButton>
                    ) : r.status === "published" ? (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "featured" })} tone="sky">
                            <Star className="size-3" /> Feature
                        </ActionButton>
                    ) : null}
                    {r.status === "hidden" ? (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "published" })}>
                            <RotateCcw className="size-3" /> Restore
                        </ActionButton>
                    ) : (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "hidden" })} tone="red">
                            <EyeOff className="size-3" /> Hide
                        </ActionButton>
                    )}
                </div>
            ),
        },
    ];

    return (
        <div>
            {!embedded && (
                <PageHeader
                    title="App directory"
                    description="OAuth apps workspaces have published. A published app opens only from its link until you feature it or 25 workspaces use it. Featured apps appear in every workspace's Integrations page; hidden apps open nowhere."
                />
            )}
            <Explorer
                activeCount={(query ? 1 : 0) + (status !== "any" ? 1 : 0)}
                onReset={() => {
                    setQuery("");
                    setStatus("any");
                }}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="App, link, publisher or URL…" />
                        </FilterGroup>
                        <FilterGroup label="Status">
                            <SelectFilter
                                value={status}
                                onChange={(v) => setStatus(v as AppListingStatus | "any")}
                                options={STATUS_OPTIONS}
                                placeholder="Any status"
                            />
                        </FilterGroup>
                    </>
                }
            >
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(r) => r.application_id}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load app listings"
                    storageKey="admin.app-listings"
                    csvName="warmbly-app-listings"
                    noun="listings"
                    emptyTitle="No published apps"
                    emptyHint="No listings match these filters."
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

            {acting && <StatusDialog item={acting.item} to={acting.to} onOpenChange={(v) => !v && setActing(null)} />}
        </div>
    );
}

function ActionButton({
    children,
    onClick,
    disabled,
    tone,
}: {
    children: React.ReactNode;
    onClick: () => void;
    disabled: boolean;
    tone?: "sky" | "red";
}) {
    return (
        <Button
            size="sm"
            variant={tone ? "default" : "outline"}
            disabled={disabled}
            onClick={(e) => {
                e.stopPropagation();
                onClick();
            }}
            className={
                tone === "sky"
                    ? "bg-sky-600 hover:bg-sky-700 text-white text-xs"
                    : tone === "red"
                      ? "bg-red-600 hover:bg-red-700 text-white text-xs"
                      : "text-xs"
            }
        >
            {children}
        </Button>
    );
}

const DIALOG_COPY: Record<AppListingStatus, { title: string; body: string; cta: string }> = {
    featured: {
        title: "Feature",
        body: "Lists it in every workspace's Integrations page with a Featured badge. Check that the install URL, website and description belong to the same product and that the permissions fit what it does. Any later edit removes the feature.",
        cta: "Feature",
    },
    published: {
        title: "Set to link only",
        body: "Removes it from the directory unless enough workspaces use it. Its link keeps working.",
        cta: "Set to link only",
    },
    hidden: {
        title: "Hide",
        body: "Takes it down everywhere: out of the directory and its link stops working. Workspaces that installed it keep their access until they revoke it. The developer sees your note.",
        cta: "Hide",
    },
};

function StatusDialog({
    item,
    to,
    onOpenChange,
}: {
    item: AdminAppListing;
    to: AppListingStatus;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [note, setNote] = useState("");
    const copy = DIALOG_COPY[to];
    const mutation = useMutation({
        mutationFn: () => setAppListingStatus(item.application_id, to, note),
        onSuccess: () => {
            toast.success(`${item.name}: ${STATUS_LABEL[to].toLowerCase()}`);
            qc.invalidateQueries({ queryKey: ["admin", "app-listings"] });
            onOpenChange(false);
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    return (
        <Dialog open onOpenChange={onOpenChange}>
            <DialogContent className="max-w-lg">
                <DialogHeader>
                    <DialogTitle>
                        {copy.title} {item.name}
                    </DialogTitle>
                    <DialogDescription>{copy.body}</DialogDescription>
                </DialogHeader>

                <div className="space-y-3 text-xs">
                    <p className="text-sm">{item.tagline}</p>
                    {item.description && (
                        <p className="whitespace-pre-line text-muted-foreground max-h-40 overflow-y-auto">{item.description}</p>
                    )}
                    <dl className="grid grid-cols-[110px_1fr] gap-x-3 gap-y-1">
                        <dt className="text-muted-foreground">Publisher</dt>
                        <dd>{item.organization_name || item.organization_id}</dd>
                        <dt className="text-muted-foreground">Install</dt>
                        <dd className="font-mono break-all">{item.install_url}</dd>
                        <dt className="text-muted-foreground">Website</dt>
                        <dd className="font-mono break-all">{item.website_url || "none"}</dd>
                        <dt className="text-muted-foreground">Permissions</dt>
                        <dd>{item.permissions.map((p) => p.name.toLowerCase()).join(", ") || "none"}</dd>
                        <dt className="text-muted-foreground">Installs</dt>
                        <dd>{item.installs.toLocaleString()}</dd>
                    </dl>
                </div>

                {to === "hidden" && (
                    <div>
                        <Label htmlFor="listing-note" className="text-xs font-medium">
                            Note to the developer (required)
                        </Label>
                        <Textarea
                            id="listing-note"
                            rows={3}
                            maxLength={1000}
                            placeholder="Why it was taken down"
                            value={note}
                            onChange={(e) => setNote(e.target.value)}
                        />
                    </div>
                )}

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => {
                            if (to === "hidden" && note.trim() === "") {
                                toast.error("Add a note so the developer knows why");
                                return;
                            }
                            mutation.mutate();
                        }}
                        disabled={mutation.isPending}
                        className={to === "hidden" ? "bg-red-600 hover:bg-red-700 text-white" : "bg-sky-600 hover:bg-sky-700 text-white"}
                    >
                        {mutation.isPending ? "Working…" : copy.cta}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
