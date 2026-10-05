// Import an archive into a workspace: choose the file, run preflight (reads
// the manifest and reports conflicts without writing), then apply. Multipart
// field names match the dashboard's Settings > Data flow: "file", "options"
// (JSON), "passphrase".

import { useEffect, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AlertTriangle, FileArchive, Loader2 } from "lucide-react";
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
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useConfirm } from "@/components/ConfirmDialog";
import { Callout, Property, PropertyList, StatusBadge, StatusDot } from "@/components/ui/kit";
import { cn } from "@/lib/utils";
import {
    createOrgImport,
    expandGroups,
    formatBytes,
    preflightOrgImport,
    totalRows,
    type OrgDataGroup,
    type OrgImportConflict,
    type OrgImportPreflight,
} from "@/lib/api/client/admin/transfers";
import { GroupPicker } from "./GroupPicker";
import { fmtDateTime } from "../fleet/format";

export function ImportArchiveDialog({
    open,
    onOpenChange,
    orgId,
    orgName,
    onStarted,
}: {
    open: boolean;
    onOpenChange: (v: boolean) => void;
    orgId: string;
    orgName: string;
    onStarted?: () => void;
}) {
    const qc = useQueryClient();
    const confirm = useConfirm();
    const fileRef = useRef<HTMLInputElement>(null);
    const [file, setFile] = useState<File | null>(null);
    const [passphrase, setPassphrase] = useState("");
    const [report, setReport] = useState<OrgImportPreflight | null>(null);
    const [groups, setGroups] = useState<Set<OrgDataGroup>>(new Set());
    const [conflict, setConflict] = useState<OrgImportConflict>("skip");

    useEffect(() => {
        if (!open) return;
        setFile(null);
        setPassphrase("");
        setReport(null);
        setGroups(new Set());
        setConflict("skip");
    }, [open]);

    function pickFile(next: File | null) {
        setFile(next);
        setReport(null);
    }

    const preflight = useMutation({
        mutationFn: () => preflightOrgImport(orgId, file!, passphrase),
        onSuccess: (r) => {
            setReport(r);
            setGroups(expandGroups(r.archive.groups));
        },
        onError: (e: Error) => {
            setReport(null);
            toast.error(e.message || "That archive could not be read");
        },
    });

    const apply = useMutation({
        mutationFn: () =>
            createOrgImport(orgId, file!, { groups: Array.from(groups), conflict_strategy: conflict }, passphrase),
        onSuccess: () => {
            toast.success(`Import started into ${orgName}`);
            qc.invalidateQueries({ queryKey: ["admin", "transfers"] });
            onStarted?.();
            onOpenChange(false);
        },
        onError: (e: Error) => toast.error(e.message || "Could not start the import"),
    });

    async function onApply() {
        if (!file || !report) return;
        const conflictTotal = Object.values(report.conflicts ?? {}).reduce((a, b) => a + b, 0);
        const ok = await confirm({
            title: `Import "${report.archive.organization_name}" into ${orgName}?`,
            description:
                conflict === "overwrite" && conflictTotal > 0
                    ? `${conflictTotal.toLocaleString()} existing row(s) in this workspace will be replaced with the archive's versions. That cannot be undone.`
                    : `The archive's contents are added to this workspace. Rows that already exist here are kept as they are.`,
            confirmLabel: "Start import",
            destructive: conflict === "overwrite",
        });
        if (ok) apply.mutate();
    }

    const busy = preflight.isPending || apply.isPending;
    const conflictTotal = Object.values(report?.conflicts ?? {}).reduce((a, b) => a + b, 0);

    return (
        <Dialog
            open={open}
            onOpenChange={(v) => {
                if (!v && busy) return;
                onOpenChange(v);
            }}
        >
            <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>Import an archive into {orgName}</DialogTitle>
                    <DialogDescription>
                        Preflight reads the archive and reports what would land, what already exists, and who has no
                        account here, without writing anything.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-5">
                    <div
                        className={cn(
                            "flex flex-col gap-3 rounded-lg border px-3.5 py-3 sm:flex-row sm:items-center",
                            file ? "border-border bg-card" : "border-dashed border-border-strong bg-muted/30",
                        )}
                    >
                        <span className="grid size-8 shrink-0 place-items-center rounded-md border border-border bg-card text-subtle-foreground">
                            <FileArchive className="size-4" />
                        </span>
                        <div className="min-w-0 flex-1">
                            <div className="truncate text-[13px] font-medium leading-tight text-foreground">
                                {file ? file.name : "Choose an archive"}
                            </div>
                            <div className="mt-0.5 text-xs leading-tight text-muted-foreground tabular-nums">
                                {file ? formatBytes(file.size) : "A .warmbly.zip exported from this or another instance."}
                            </div>
                        </div>
                        <input
                            ref={fileRef}
                            type="file"
                            accept=".zip,application/zip"
                            className="hidden"
                            onChange={(e) => pickFile(e.target.files?.[0] ?? null)}
                        />
                        <Button size="sm" variant="outline" onClick={() => fileRef.current?.click()} disabled={busy}>
                            {file ? "Choose another" : "Choose file"}
                        </Button>
                    </div>

                    {file && (
                        <div className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
                            <div className="space-y-1.5">
                                <Label htmlFor="imp-pass" className="text-xs font-medium text-muted-foreground">
                                    Export passphrase{" "}
                                    <span className="font-normal text-subtle-foreground">(only if the archive carries credentials)</span>
                                </Label>
                                <Input
                                    id="imp-pass"
                                    type="password"
                                    autoComplete="current-password"
                                    value={passphrase}
                                    onChange={(e) => {
                                        setPassphrase(e.target.value);
                                        setReport(null);
                                    }}
                                />
                            </div>
                            <Button
                                variant={report ? "outline" : "default"}
                                onClick={() => preflight.mutate()}
                                disabled={busy}
                            >
                                {preflight.isPending && <Loader2 className="animate-spin" />}
                                {report ? "Check again" : "Check archive"}
                            </Button>
                        </div>
                    )}

                    {report && (
                        <div className="space-y-5">
                            <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
                                <div className="flex items-center justify-between gap-3 border-b border-border px-3.5 py-2.5">
                                    <span className="text-[13px] font-medium text-foreground">Preflight</span>
                                    <StatusBadge tone={conflictTotal === 0 ? "success" : "warning"} dot>
                                        {conflictTotal === 0 ? "No conflicts" : `${conflictTotal.toLocaleString()} conflicts`}
                                    </StatusBadge>
                                </div>
                                <div className="grid gap-x-6 px-3.5 py-1 sm:grid-cols-2">
                                    <PropertyList>
                                        <Property label="Source workspace">{report.archive.organization_name}</Property>
                                        <Property label="Source instance">{report.archive.source_instance || "—"}</Property>
                                        <Property label="Rows">
                                            <span className="tabular-nums">
                                                {totalRows(report.archive.row_counts).toLocaleString()}
                                            </span>
                                        </Property>
                                        <Property label="Credentials">
                                            <StatusDot
                                                tone={
                                                    !report.archive.has_secrets
                                                        ? "neutral"
                                                        : report.secrets_unsealed
                                                          ? "success"
                                                          : "danger"
                                                }
                                                className="items-start [&>span:first-child]:mt-1.5"
                                            >
                                                {!report.archive.has_secrets
                                                    ? "not in archive; mailboxes will need reconnecting"
                                                    : report.secrets_unsealed
                                                      ? "unsealed; mailboxes reconnect automatically"
                                                      : "sealed; wrong or missing passphrase"}
                                            </StatusDot>
                                        </Property>
                                    </PropertyList>
                                    <PropertyList className="border-t border-border sm:border-t-0">
                                        <Property label="Exported">{fmtDateTime(report.archive.exported_at)}</Property>
                                        <Property label="App version">
                                            <span className="font-mono text-xs">{report.archive.source_app_version || "—"}</span>
                                        </Property>
                                        <Property label="Blobs">
                                            <span className="tabular-nums">{String(report.archive.blob_count)}</span>
                                        </Property>
                                        <Property label="Conflicts">
                                            {conflictTotal === 0 ? "none" : `${conflictTotal.toLocaleString()} existing row(s)`}
                                        </Property>
                                    </PropertyList>
                                </div>
                                {((report.unknown_members?.length ?? 0) > 0 || (report.skipped_tables?.length ?? 0) > 0) && (
                                    <div className="space-y-1.5 border-t border-border px-3.5 py-2.5 text-xs leading-relaxed text-muted-foreground">
                                        {(report.unknown_members?.length ?? 0) > 0 && (
                                            <p>
                                                {report.unknown_members!.length} member(s) have no account on this instance and arrive as
                                                pending invitations: {report.unknown_members!.map((m) => m.email).join(", ")}.
                                            </p>
                                        )}
                                        {(report.skipped_tables?.length ?? 0) > 0 && (
                                            <p>Skipped (unknown here): {report.skipped_tables!.join(", ")}.</p>
                                        )}
                                    </div>
                                )}
                            </div>

                            {(report.warnings?.length ?? 0) > 0 && (
                                <Callout tone="warning" icon={AlertTriangle}>
                                    <ul className="space-y-1">
                                        {report.warnings!.map((w, i) => (
                                            <li key={i}>{w}</li>
                                        ))}
                                    </ul>
                                </Callout>
                            )}

                            <div className="space-y-1.5">
                                <Label className="text-xs font-medium text-muted-foreground">Groups to apply</Label>
                                <GroupPicker selected={groups} onChange={setGroups} available={report.archive.groups} disabled={busy} />
                            </div>

                            <div className="space-y-1.5">
                                <Label className="text-xs font-medium text-muted-foreground">When a row already exists here</Label>
                                <Select value={conflict} onValueChange={(v) => setConflict(v as OrgImportConflict)}>
                                    <SelectTrigger className="w-full sm:w-80">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="skip">Skip: keep the row that is already here</SelectItem>
                                        <SelectItem value="overwrite">
                                            Overwrite: replace it with the archive's version
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                    )}
                </div>

                <DialogFooter>
                    <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={busy}>
                        Cancel
                    </Button>
                    <Button
                        variant={conflict === "overwrite" ? "destructive" : "default"}
                        onClick={() => void onApply()}
                        disabled={!report || busy}
                    >
                        {apply.isPending ? "Starting…" : "Apply import"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
