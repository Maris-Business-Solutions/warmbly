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
import { Button } from "@/components/ui/button";
import { Property, PropertyList, StatusBadge } from "@/components/ui/kit";
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
import { TONE_TEXT, type Tone } from "@/lib/tones";

const STATUS_LABEL: Record<AppListingStatus, string> = {
    published: "Link only",
    featured: "Featured",
    hidden: "Hidden",
};

const STATUS_TONE: Record<AppListingStatus, Tone> = {
    published: "neutral",
    featured: "accent",
    hidden: "danger",
};

const STATUS_OPTIONS = [
    { value: "any", label: "Any status" },
    { value: "published", label: "Link only" },
    { value: "featured", label: "Featured" },
    { value: "hidden", label: "Hidden" },
];

// Logo or initial tile, shared with the Apps tab.
export function AppAvatar({ name, logoUrl }: { name: string; logoUrl?: string | null }) {
    return logoUrl ? (
        <img src={logoUrl} alt="" className="size-7 shrink-0 rounded-md border border-border bg-muted object-cover" />
    ) : (
        <div className="flex size-7 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-xs font-semibold text-muted-foreground">
            {(name[0] ?? "?").toUpperCase()}
        </div>
    );
}

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
                <div className="flex min-w-0 items-center gap-2.5">
                    <AppAvatar name={r.name} logoUrl={r.logo_url} />
                    <div className="min-w-0">
                        <div className="truncate text-[13px] font-medium text-foreground">{r.name}</div>
                        <div className="truncate font-mono text-[11px] text-subtle-foreground">{r.slug}</div>
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
                    className="text-[13px] text-foreground hover:text-[var(--admin-accent-strong)] hover:underline"
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
                    <div className="truncate text-[13px] text-foreground" title={r.tagline}>
                        {r.tagline}
                    </div>
                    <div className="text-xs text-muted-foreground capitalize">{r.category}</div>
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
                    className="inline-flex items-center gap-1 font-mono text-xs text-muted-foreground hover:text-foreground hover:underline"
                >
                    {hostOf(r.install_url)}
                    <ExternalLink className="size-3 text-subtle-foreground" />
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
                    <span className="text-[13px] tabular-nums" title={r.permissions.map((p) => p.name.toLowerCase()).join(", ")}>
                        {r.permissions.length}
                        {writes > 0 && <span className={`text-xs ${TONE_TEXT.warning}`}> ({writes} write)</span>}
                    </span>
                );
            },
            csv: (r) => r.permissions.map((p) => p.name).join(" "),
        },
        {
            id: "installs",
            header: "Installs",
            align: "right",
            cell: (r) => <span className="text-[13px] tabular-nums">{r.installs.toLocaleString()}</span>,
            csv: (r) => r.installs,
        },
        {
            id: "status",
            header: "Status",
            cell: (r) => (
                <div className="space-y-1 py-1">
                    <StatusBadge tone={STATUS_TONE[r.status]} dot>
                        {STATUS_LABEL[r.status]}
                    </StatusBadge>
                    {r.status === "published" && r.listed && (
                        <div className="text-xs text-muted-foreground">Listed by installs</div>
                    )}
                    {r.app_status !== "active" && <div className={`text-xs ${TONE_TEXT.warning}`}>App {r.app_status}</div>}
                    {r.status === "hidden" && r.status_note && (
                        <div className="max-w-xs truncate text-xs text-muted-foreground" title={r.status_note}>
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
                <div className="inline-flex items-center gap-1.5 whitespace-nowrap">
                    {r.status === "featured" ? (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "published" })}>
                            <StarOff className="size-3" /> Unfeature
                        </ActionButton>
                    ) : r.status === "published" ? (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "featured" })} tone="primary">
                            <Star className="size-3" /> Feature
                        </ActionButton>
                    ) : null}
                    {r.status === "hidden" ? (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "published" })}>
                            <RotateCcw className="size-3" /> Restore
                        </ActionButton>
                    ) : (
                        <ActionButton disabled={!canManage} onClick={() => setActing({ item: r, to: "hidden" })} tone="danger">
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
    tone?: "primary" | "danger";
}) {
    return (
        <Button
            size="xs"
            variant={tone === "primary" ? "default" : "outline"}
            disabled={disabled}
            onClick={(e) => {
                e.stopPropagation();
                onClick();
            }}
            className={tone === "danger" ? TONE_TEXT.danger : undefined}
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

                <div className="space-y-3">
                    <div className="flex items-start gap-2.5">
                        <AppAvatar name={item.name} logoUrl={item.logo_url} />
                        <div className="min-w-0">
                            <p className="text-[13px] font-medium text-foreground">{item.tagline}</p>
                            {item.description && (
                                <p className="mt-1 max-h-40 overflow-y-auto text-[12.5px] leading-relaxed whitespace-pre-line text-muted-foreground">
                                    {item.description}
                                </p>
                            )}
                        </div>
                    </div>
                    <PropertyList className="surface-lit rounded-xl border border-border bg-card px-3">
                        <Property label="Publisher">{item.organization_name || item.organization_id}</Property>
                        <Property label="Install">
                            <span className="font-mono text-xs break-all">{item.install_url}</span>
                        </Property>
                        <Property label="Website">
                            <span className="font-mono text-xs break-all">{item.website_url || "none"}</span>
                        </Property>
                        <Property label="Permissions">{item.permissions.map((p) => p.name.toLowerCase()).join(", ") || "none"}</Property>
                        <Property label="Installs">
                            <span className="tabular-nums">{item.installs.toLocaleString()}</span>
                        </Property>
                    </PropertyList>
                </div>

                {to === "hidden" && (
                    <div className="space-y-1.5">
                        <Label htmlFor="listing-note" className="text-xs font-medium text-muted-foreground">
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
                        variant={to === "hidden" ? "destructive" : "default"}
                    >
                        {mutation.isPending ? "Working…" : copy.cta}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
