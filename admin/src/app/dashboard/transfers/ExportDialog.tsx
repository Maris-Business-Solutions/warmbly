// Start an archive build for a workspace. With `org` fixed (organization
// detail page) the picker is hidden; without it the operator searches first.
// Secrets only travel under a passphrase, which is used once and never stored.

import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AlertTriangle, KeyRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    createOrgExport,
    expandGroups,
    MIN_EXPORT_PASSPHRASE,
    ORG_DATA_GROUP_CATALOG,
    type OrgDataGroup,
} from "@/lib/api/client/admin/transfers";
import { OrgPicker, type PickedOrg } from "../fleet/OrgPicker";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { GroupPicker } from "./GroupPicker";

function defaultGroups(): Set<OrgDataGroup> {
    return expandGroups(ORG_DATA_GROUP_CATALOG.filter((g) => !g.heavy).map((g) => g.key));
}

export function ExportDialog({
    open,
    onOpenChange,
    org,
    onStarted,
}: {
    open: boolean;
    onOpenChange: (v: boolean) => void;
    org?: { id: string; name: string } | null;
    onStarted?: () => void;
}) {
    const qc = useQueryClient();
    const [picked, setPicked] = useState<PickedOrg | null>(null);
    const [groups, setGroups] = useState<Set<OrgDataGroup>>(defaultGroups);
    const [secrets, setSecrets] = useState(false);
    const [passphrase, setPassphrase] = useState("");
    const [confirmPass, setConfirmPass] = useState("");

    useEffect(() => {
        if (!open) return;
        setPicked(null);
        setGroups(defaultGroups());
        setSecrets(false);
        setPassphrase("");
        setConfirmPass("");
    }, [open]);

    const target = org ?? (picked ? { id: picked.id, name: picked.name } : null);
    const passOk = !secrets || (passphrase.length >= MIN_EXPORT_PASSPHRASE && passphrase === confirmPass);
    const heavyOn = ORG_DATA_GROUP_CATALOG.filter((g) => g.heavy && groups.has(g.key));

    const mutation = useMutation({
        mutationFn: () =>
            createOrgExport(target!.id, {
                groups: Array.from(groups),
                include_secrets: secrets,
                passphrase: secrets ? passphrase : undefined,
            }),
        onSuccess: () => {
            toast.success(`Export started for ${target?.name}`);
            qc.invalidateQueries({ queryKey: ["admin", "transfers"] });
            onStarted?.();
            onOpenChange(false);
        },
        onError: (e: Error) => toast.error(e.message || "Could not start the export"),
    });

    return (
        <Dialog
            open={open}
            onOpenChange={(v) => {
                if (!v && mutation.isPending) return;
                onOpenChange(v);
            }}
        >
            <DialogContent
                className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl"
                onEscapeKeyDown={(e) => {
                    if (document.querySelector("[data-floating]")) e.preventDefault();
                }}
            >
                <DialogHeader>
                    <DialogTitle>Export a workspace</DialogTitle>
                    <DialogDescription>
                        Builds the same archive an owner gets from Settings &gt; Data. It stays downloadable for 7 days.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-5">
                    {!org && (
                        <div className="space-y-1.5">
                            <Label className="text-xs font-medium text-muted-foreground">Workspace</Label>
                            <OrgPicker value={picked} onChange={setPicked} autoFocus />
                        </div>
                    )}

                    <div className="space-y-1.5">
                        <Label className="text-xs font-medium text-muted-foreground">Data groups</Label>
                        <GroupPicker selected={groups} onChange={setGroups} />
                        {heavyOn.length > 0 && (
                            <p className={cn("flex items-start gap-1.5 text-xs", TONE_TEXT.warning)}>
                                <AlertTriangle className="mt-px size-3.5 shrink-0" />
                                {heavyOn.map((g) => g.label).join(", ")} can multiply the archive size on a busy workspace.
                            </p>
                        )}
                    </div>

                    <div className="overflow-hidden surface-lit rounded-xl border border-border bg-card">
                        <label className="flex cursor-pointer items-start justify-between gap-4 px-3.5 py-3">
                            <span className="min-w-0">
                                <span className="flex items-center gap-1.5 text-[13px] font-medium text-foreground">
                                    <KeyRound className="size-3.5 text-subtle-foreground" />
                                    Include credentials
                                </span>
                                <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">
                                    Re-seals mailbox and integration credentials into the archive under a passphrase, so
                                    the destination brings mailboxes back up without every user reconnecting.
                                </span>
                            </span>
                            <Switch checked={secrets} onCheckedChange={setSecrets} className="mt-0.5" />
                        </label>
                        {secrets && (
                            <div className="grid gap-3 border-t border-border bg-muted/30 px-3.5 py-3 sm:grid-cols-2">
                                <div className="space-y-1.5">
                                    <Label htmlFor="exp-pass" className="text-xs font-medium text-muted-foreground">
                                        Passphrase
                                    </Label>
                                    <Input
                                        id="exp-pass"
                                        aria-describedby="exp-pass-help"
                                        aria-invalid={!!passphrase && passphrase.length < MIN_EXPORT_PASSPHRASE}
                                        type="password"
                                        autoComplete="new-password"
                                        value={passphrase}
                                        onChange={(e) => setPassphrase(e.target.value)}
                                    />
                                </div>
                                <div className="space-y-1.5">
                                    <Label htmlFor="exp-pass2" className="text-xs font-medium text-muted-foreground">
                                        Repeat passphrase
                                    </Label>
                                    <Input
                                        id="exp-pass2"
                                        aria-describedby="exp-pass-help"
                                        aria-invalid={!!confirmPass && passphrase !== confirmPass}
                                        type="password"
                                        autoComplete="new-password"
                                        value={confirmPass}
                                        onChange={(e) => setConfirmPass(e.target.value)}
                                    />
                                </div>
                                <p id="exp-pass-help" className="text-xs leading-relaxed text-muted-foreground sm:col-span-2">
                                    At least {MIN_EXPORT_PASSPHRASE} characters. It is never stored: whoever imports the
                                    archive needs it, and there is no recovery.
                                    {passphrase && passphrase.length < MIN_EXPORT_PASSPHRASE && (
                                        <span className={cn("ml-1", TONE_TEXT.danger)}>Too short.</span>
                                    )}
                                    {passphrase.length >= MIN_EXPORT_PASSPHRASE && confirmPass && passphrase !== confirmPass && (
                                        <span className={cn("ml-1", TONE_TEXT.danger)}>Passphrases differ.</span>
                                    )}
                                </p>
                            </div>
                        )}
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={mutation.isPending}>
                        Cancel
                    </Button>
                    <Button onClick={() => mutation.mutate()} disabled={!target || !passOk || mutation.isPending}>
                        {mutation.isPending ? "Starting…" : "Start export"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
