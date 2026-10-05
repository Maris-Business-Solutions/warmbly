// One workspace's exports and imports, with the export dialog and the
// import flow. Polls at 15s only while a job is queued or running.

import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Download, PackageOpen, Trash2, Upload, type LucideIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, Section } from "@/components/ui/kit";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { ErrorState } from "@/components/ErrorState";
import { useConfirm } from "@/components/ConfirmDialog";
import {
    deleteOrgExport,
    downloadOrgExport,
    formatBytes,
    isTransferActive,
    listOrgExports,
    listOrgImports,
    saveBlob,
    totalRows,
    type OrgExportJob,
    type OrgImportJob,
} from "@/lib/api/client/admin/transfers";
import { ExportDialog } from "./ExportDialog";
import { ImportArchiveDialog } from "./ImportArchiveDialog";
import { StatusPill } from "./TransferPills";
import { fmtDateTime } from "../fleet/format";

const POLL = 15_000;

export function OrgTransferTab({ orgId, orgName }: { orgId: string; orgName: string }) {
    const qc = useQueryClient();
    const confirm = useConfirm();
    const [exportOpen, setExportOpen] = useState(false);
    const [importOpen, setImportOpen] = useState(false);

    const exportsQ = useQuery({
        queryKey: ["admin", "transfers", "org", orgId, "exports"],
        queryFn: () => listOrgExports(orgId),
        refetchInterval: (q) => ((q.state.data?.data ?? []).some((j) => isTransferActive(j.status)) ? POLL : false),
    });
    const importsQ = useQuery({
        queryKey: ["admin", "transfers", "org", orgId, "imports"],
        queryFn: () => listOrgImports(orgId),
        refetchInterval: (q) => ((q.state.data?.data ?? []).some((j) => isTransferActive(j.status)) ? POLL : false),
    });

    const invalidate = () => qc.invalidateQueries({ queryKey: ["admin", "transfers"] });

    const [downloading, setDownloading] = useState<string | null>(null);
    async function onDownload(job: OrgExportJob) {
        setDownloading(job.id);
        try {
            const { blob, filename } = await downloadOrgExport(orgId, job.id, orgName);
            saveBlob(blob, filename);
        } catch (e) {
            toast.error((e as Error).message || "Download failed");
        } finally {
            setDownloading(null);
        }
    }

    const del = useMutation({
        mutationFn: (id: string) => deleteOrgExport(orgId, id),
        onSuccess: () => {
            toast.success("Archive deleted");
            invalidate();
        },
        onError: (e: Error) => toast.error(e.message || "Delete failed"),
    });

    async function onDelete(job: OrgExportJob) {
        const ok = await confirm({
            title: "Delete this archive?",
            description: "The stored file is removed. The job stays in the history as deleted; run another export to rebuild it.",
            confirmLabel: "Delete",
            destructive: true,
        });
        if (ok) del.mutate(job.id);
    }

    const exports = exportsQ.data?.data ?? [];
    const imports = importsQ.data?.data ?? [];

    return (
        <div>
            <p className="mb-8 max-w-2xl text-[13px] leading-relaxed text-muted-foreground">
                The same archives the owner builds from Settings &gt; Data, started here on their behalf. Finished
                exports stay downloadable for 7 days.
            </p>

            <Section
                title={<TitleCount label="Exports" count={exports.length} />}
                actions={
                    <Button size="sm" onClick={() => setExportOpen(true)}>
                        <PackageOpen />
                        Export
                    </Button>
                }
            >
                {exportsQ.isLoading ? (
                    <Skeleton className="h-24 w-full rounded-lg" />
                ) : exportsQ.error ? (
                    <ErrorState error={exportsQ.error} title="Failed to load exports" onRetry={() => exportsQ.refetch()} />
                ) : exports.length === 0 ? (
                    <Empty icon={PackageOpen} title="No exports yet" hint="Start one above to build a portable archive of this workspace." />
                ) : (
                    <TableShell>
                        <thead>
                            <tr className={HEAD_ROW}>
                                <th className={cn(TH, "pl-4")}>Status</th>
                                <th className={TH}>Groups</th>
                                <th className={TH}>Secrets</th>
                                <th className={cn(TH, "text-right")}>Rows</th>
                                <th className={cn(TH, "text-right")}>Size</th>
                                <th className={TH}>Started</th>
                                <th className={TH}>Expires</th>
                                <th className={cn(TH, "pr-4 text-right")}>
                                    <span className="sr-only">Actions</span>
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {exports.map((j) => (
                                <tr key={j.id} className={BODY_ROW}>
                                    <td className={cn(TD, "pl-4")}>
                                        <StatusPill status={j.status} progress={j.progress_percent} stage={j.progress_stage} />
                                        {j.error_message && (
                                            <div className={cn("mt-0.5 max-w-xs text-xs", TONE_TEXT.danger)}>{j.error_message}</div>
                                        )}
                                    </td>
                                    <td className={cn(TD, "tabular-nums")}>{j.groups?.length ?? 0}</td>
                                    <td className={cn(TD, "text-muted-foreground")}>{j.include_secrets ? "yes" : "no"}</td>
                                    <td className={cn(TD, "text-right tabular-nums")}>{totalRows(j.row_counts).toLocaleString()}</td>
                                    <td className={cn(TD, "text-right tabular-nums")}>{formatBytes(j.archive_bytes)}</td>
                                    <td className={cn(TD, "text-muted-foreground")}>{fmtDateTime(j.started_at ?? j.created_at)}</td>
                                    <td className={cn(TD, "text-muted-foreground")}>{fmtDateTime(j.expires_at)}</td>
                                    <td className={cn(TD, "pr-4 text-right")}>
                                        <div className="flex justify-end gap-1">
                                            <Button
                                                size="xs"
                                                variant="outline"
                                                disabled={j.status !== "completed" || downloading === j.id}
                                                onClick={() => void onDownload(j)}
                                            >
                                                <Download />
                                                {downloading === j.id ? "Fetching…" : "Download"}
                                            </Button>
                                            <Button
                                                size="icon-xs"
                                                variant="ghost"
                                                className={DANGER_ICON}
                                                disabled={isTransferActive(j.status) || del.isPending}
                                                onClick={() => void onDelete(j)}
                                                aria-label="Delete archive"
                                            >
                                                <Trash2 />
                                            </Button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </TableShell>
                )}
            </Section>

            <Section
                title={<TitleCount label="Imports" count={imports.length} />}
                actions={
                    <Button size="sm" variant="outline" onClick={() => setImportOpen(true)}>
                        <Upload />
                        Import archive
                    </Button>
                }
            >
                {importsQ.isLoading ? (
                    <Skeleton className="h-24 w-full rounded-lg" />
                ) : importsQ.error ? (
                    <ErrorState error={importsQ.error} title="Failed to load imports" onRetry={() => importsQ.refetch()} />
                ) : imports.length === 0 ? (
                    <Empty icon={Upload} title="No imports yet" hint="Apply an archive from another instance (or an older export) above." />
                ) : (
                    <TableShell>
                        <thead>
                            <tr className={HEAD_ROW}>
                                <th className={cn(TH, "pl-4")}>Status</th>
                                <th className={TH}>Source</th>
                                <th className={TH}>Groups</th>
                                <th className={TH}>Conflicts</th>
                                <th className={cn(TH, "text-right")}>Rows</th>
                                <th className={cn(TH, "text-right")}>Size</th>
                                <th className={TH}>Started</th>
                                <th className={cn(TH, "pr-4")}>Completed</th>
                            </tr>
                        </thead>
                        <tbody>
                            {imports.map((j: OrgImportJob) => (
                                <tr key={j.id} className={BODY_ROW}>
                                    <td className={cn(TD, "pl-4")}>
                                        <StatusPill status={j.status} progress={j.progress_percent} stage={j.progress_stage} />
                                        {j.error_message && (
                                            <div className={cn("mt-0.5 max-w-xs text-xs", TONE_TEXT.danger)}>{j.error_message}</div>
                                        )}
                                        {(j.warnings?.length ?? 0) > 0 && (
                                            <div className={cn("mt-0.5 max-w-xs text-xs", TONE_TEXT.warning)}>{j.warnings!.join(" · ")}</div>
                                        )}
                                    </td>
                                    <td className={TD}>
                                        {j.source_manifest ? (
                                            <>
                                                <div className="text-foreground">{j.source_manifest.organization_name}</div>
                                                <div className="text-xs text-muted-foreground">
                                                    {j.source_manifest.source_instance || "unknown instance"}
                                                </div>
                                            </>
                                        ) : (
                                            <span className="text-subtle-foreground">—</span>
                                        )}
                                    </td>
                                    <td className={cn(TD, "tabular-nums")}>{j.groups?.length ?? 0}</td>
                                    <td className={cn(TD, "text-muted-foreground")}>{j.conflict_strategy}</td>
                                    <td className={cn(TD, "text-right tabular-nums")}>{totalRows(j.row_counts).toLocaleString()}</td>
                                    <td className={cn(TD, "text-right tabular-nums")}>{formatBytes(j.archive_bytes)}</td>
                                    <td className={cn(TD, "text-muted-foreground")}>{fmtDateTime(j.started_at ?? j.created_at)}</td>
                                    <td className={cn(TD, "pr-4 text-muted-foreground")}>{fmtDateTime(j.completed_at)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </TableShell>
                )}
            </Section>

            <ExportDialog open={exportOpen} onOpenChange={setExportOpen} org={{ id: orgId, name: orgName }} onStarted={invalidate} />
            <ImportArchiveDialog open={importOpen} onOpenChange={setImportOpen} orgId={orgId} orgName={orgName} onStarted={invalidate} />
        </div>
    );
}

const HEAD_ROW = "h-9 border-b border-border text-left text-xs text-muted-foreground";
const TH = "px-3 font-medium whitespace-nowrap";
const BODY_ROW = "border-b border-border/70 transition-colors last:border-b-0 hover:bg-accent/50";
const TD = "h-10 px-3 py-2 align-middle whitespace-nowrap";
const DANGER_ICON =
    "text-muted-foreground hover:bg-[color-mix(in_oklab,var(--destructive)_10%,transparent)] hover:text-destructive";

function TableShell({ children }: { children: ReactNode }) {
    return (
        <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
            <div className="overflow-x-auto">
                <table className="w-full text-[13px]">{children}</table>
            </div>
        </div>
    );
}

function TitleCount({ label, count }: { label: string; count: number }) {
    return (
        <span className="inline-flex items-center gap-1.5">
            {label}
            {count > 0 && <span className="font-normal text-muted-foreground tabular-nums">{count}</span>}
        </span>
    );
}

function Empty({ icon, title, hint }: { icon: LucideIcon; title: string; hint: string }) {
    return (
        <div className="rounded-lg border border-dashed border-border">
            <EmptyState icon={icon} title={title} hint={hint} className="py-10" />
        </div>
    );
}
