// Community app directory review queue. A published app is reachable by its
// link with an unverified warning; verifying it adds it to every workspace's
// Integrations page, rejecting hides it until the developer edits it. Any edit
// to a verified listing or to its app returns it here as unverified.

import { useEffect, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { BadgeCheck, ExternalLink, XCircle } from "lucide-react";
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
import { listAppListings, rejectAppListing, verifyAppListing } from "@/lib/api/client/admin/appListings";
import type { AdminAppListing, AppListingVerification } from "@/lib/api/models/admin";

const STATUS_TONE: Record<AppListingVerification, string> = {
    unverified: "border-amber-300 text-amber-700 bg-amber-50",
    verified: "border-emerald-300 text-emerald-700 bg-emerald-50",
    rejected: "border-red-300 text-red-700 bg-red-50",
};

const STATUS_OPTIONS = [
    { value: "unverified", label: "Awaiting review" },
    { value: "verified", label: "Verified" },
    { value: "rejected", label: "Rejected" },
    { value: "any", label: "Any status" },
];

function hostOf(url: string): string {
    try {
        return new URL(url).host;
    } catch {
        return url;
    }
}

export default function AppListingsPage() {
    const canManage = useAdminPerm(AdminPerm.ManageOrganizations);
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState<AppListingVerification | "any">("unverified");
    const pager = useCursorPager();
    const { reset } = pager;
    const [reviewing, setReviewing] = useState<{ item: AdminAppListing; mode: "verify" | "reject" } | null>(null);

    const filterKey = JSON.stringify({ query, status });
    useEffect(() => {
        reset();
    }, [filterKey, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "app-listings", filterKey, pager.cursor],
        queryFn: () =>
            listAppListings({
                q: query.trim() || undefined,
                verification: status === "any" ? "" : status,
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
                    <Badge variant="outline" className={`text-[10px] ${STATUS_TONE[r.verification]}`}>
                        {r.verification}
                    </Badge>
                    {r.app_status !== "active" && (
                        <div className="text-[10px] text-muted-foreground mt-1">app {r.app_status}</div>
                    )}
                    {r.review_note && (
                        <div className="text-[10px] text-muted-foreground mt-1 max-w-xs truncate" title={r.review_note}>
                            "{r.review_note}"
                        </div>
                    )}
                </div>
            ),
            csv: (r) => r.verification,
        },
        {
            id: "submitted",
            header: "Submitted",
            cell: (r) => (
                <span className="text-xs text-muted-foreground">{new Date(r.submitted_at).toLocaleDateString()}</span>
            ),
            csv: (r) => r.submitted_at,
        },
        {
            id: "actions",
            header: "",
            align: "right",
            cell: (r) => (
                <div className="space-x-1.5 whitespace-nowrap">
                    <Button
                        size="sm"
                        disabled={!canManage || r.verification === "verified"}
                        onClick={(e) => {
                            e.stopPropagation();
                            setReviewing({ item: r, mode: "verify" });
                        }}
                        className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs disabled:bg-zinc-200"
                    >
                        <BadgeCheck className="size-3" /> Verify
                    </Button>
                    <Button
                        size="sm"
                        disabled={!canManage || r.verification === "rejected"}
                        onClick={(e) => {
                            e.stopPropagation();
                            setReviewing({ item: r, mode: "reject" });
                        }}
                        className="bg-red-600 hover:bg-red-700 text-white text-xs disabled:bg-zinc-200"
                    >
                        <XCircle className="size-3" /> {r.verification === "verified" ? "Remove" : "Reject"}
                    </Button>
                </div>
            ),
        },
    ];

    return (
        <div>
            <PageHeader
                title="App directory"
                description="OAuth apps workspaces have published to the community directory. Unverified apps open only from their link, with a warning; verifying one lists it in every workspace's Integrations page."
            />
            <Explorer
                activeCount={(query ? 1 : 0) + (status !== "unverified" ? 1 : 0)}
                onReset={() => {
                    setQuery("");
                    setStatus("unverified");
                }}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="App, link, publisher or URL…" />
                        </FilterGroup>
                        <FilterGroup label="Status">
                            <SelectFilter
                                value={status}
                                onChange={(v) => setStatus(v as AppListingVerification | "any")}
                                options={STATUS_OPTIONS}
                                placeholder="Awaiting review"
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
                    onRowClick={canManage ? (r) => setReviewing({ item: r, mode: r.verification === "verified" ? "reject" : "verify" }) : undefined}
                    storageKey="admin.app-listings"
                    csvName="warmbly-app-listings"
                    noun="listings"
                    emptyTitle="Nothing to review"
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

            {reviewing && (
                <ReviewDialog
                    item={reviewing.item}
                    mode={reviewing.mode}
                    onModeChange={(mode) => setReviewing({ item: reviewing.item, mode })}
                    onOpenChange={(v) => !v && setReviewing(null)}
                />
            )}
        </div>
    );
}

function ReviewDialog({
    item,
    mode,
    onModeChange,
    onOpenChange,
}: {
    item: AdminAppListing;
    mode: "verify" | "reject";
    onModeChange: (m: "verify" | "reject") => void;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [note, setNote] = useState("");
    const mutation = useMutation({
        mutationFn: () =>
            mode === "verify" ? verifyAppListing(item.application_id, note) : rejectAppListing(item.application_id, note),
        onSuccess: () => {
            toast.success(mode === "verify" ? `${item.name} verified` : `${item.name} rejected`);
            qc.invalidateQueries({ queryKey: ["admin", "app-listings"] });
            onOpenChange(false);
        },
        onError: (err: Error) => toast.error(err.message || "Action failed"),
    });

    return (
        <Dialog open onOpenChange={onOpenChange}>
            <DialogContent className="max-w-lg">
                <DialogHeader>
                    <DialogTitle>{item.name}</DialogTitle>
                    <DialogDescription>
                        Published by {item.organization_name || item.organization_id}. Check that the install URL, website and
                        description belong to the same product, and that the permissions fit what it says it does.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-3 text-xs">
                    <p className="text-sm">{item.tagline}</p>
                    {item.description && (
                        <p className="whitespace-pre-line text-muted-foreground max-h-40 overflow-y-auto">{item.description}</p>
                    )}
                    <dl className="grid grid-cols-[110px_1fr] gap-x-3 gap-y-1">
                        <dt className="text-muted-foreground">Install</dt>
                        <dd className="font-mono break-all">{item.install_url}</dd>
                        <dt className="text-muted-foreground">Website</dt>
                        <dd className="font-mono break-all">{item.website_url || "none"}</dd>
                        {item.support_url && (
                            <>
                                <dt className="text-muted-foreground">Support</dt>
                                <dd className="font-mono break-all">{item.support_url}</dd>
                            </>
                        )}
                        {item.privacy_url && (
                            <>
                                <dt className="text-muted-foreground">Privacy</dt>
                                <dd className="font-mono break-all">{item.privacy_url}</dd>
                            </>
                        )}
                        <dt className="text-muted-foreground">Permissions</dt>
                        <dd>{item.permissions.map((p) => p.name.toLowerCase()).join(", ") || "none"}</dd>
                    </dl>
                </div>

                <div className="flex gap-1.5">
                    {(["verify", "reject"] as const).map((m) => (
                        <Button
                            key={m}
                            size="sm"
                            variant={mode === m ? "default" : "outline"}
                            onClick={() => onModeChange(m)}
                            className="text-xs"
                        >
                            {m === "verify" ? "Verify" : item.verification === "verified" ? "Remove from directory" : "Reject"}
                        </Button>
                    ))}
                </div>

                <div>
                    <Label htmlFor="review-note" className="text-xs font-medium">
                        Note to the developer {mode === "reject" ? "(required)" : "(optional)"}
                    </Label>
                    <Textarea
                        id="review-note"
                        rows={3}
                        maxLength={1000}
                        placeholder={mode === "reject" ? "What needs to change before it can be listed" : "Optional"}
                        value={note}
                        onChange={(e) => setNote(e.target.value)}
                    />
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => {
                            if (mode === "reject" && note.trim() === "") {
                                toast.error("Add a note so the developer knows what to change");
                                return;
                            }
                            mutation.mutate();
                        }}
                        disabled={mutation.isPending}
                        className={
                            mode === "verify" ? "bg-emerald-600 hover:bg-emerald-700 text-white" : "bg-red-600 hover:bg-red-700 text-white"
                        }
                    >
                        {mutation.isPending ? "Working…" : mode === "verify" ? "Verify" : "Reject"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
