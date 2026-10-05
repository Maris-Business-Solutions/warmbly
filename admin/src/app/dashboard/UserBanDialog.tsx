// Ban / unban dialog. Same component handles both actions; the
// caller passes `mode` based on current ban status. Reason is required
// so the audit trail always carries it.

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { banUser, unbanUser } from "@/lib/api/client/admin/users";

// Mirror of internal/models/admin.go BanScope constants; keep in sync
// with the Go enum and the 000045_ban_scope migration.
const BAN_SCOPE_LOGIN = 1;
const BAN_SCOPE_ORG_CREATE = 2;
const BAN_SCOPE_SEND = 4;

export function UserBanDialog({
    userId,
    userEmail,
    mode,
    open,
    onOpenChange,
}: {
    userId: string;
    userEmail: string;
    mode: "ban" | "unban";
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [reason, setReason] = useState("");
    const [scopeLogin, setScopeLogin] = useState(true);
    const [scopeOrgCreate, setScopeOrgCreate] = useState(false);
    const [scopeSend, setScopeSend] = useState(false);

    const mutation = useMutation({
        mutationFn: () => {
            if (mode === "ban") {
                const scope =
                    (scopeLogin ? BAN_SCOPE_LOGIN : 0) +
                    (scopeOrgCreate ? BAN_SCOPE_ORG_CREATE : 0) +
                    (scopeSend ? BAN_SCOPE_SEND : 0);
                return banUser(userId, { reason, scope });
            }
            return unbanUser(userId, { reason });
        },
        onSuccess: () => {
            toast.success(mode === "ban" ? "User banned" : "User unbanned");
            qc.invalidateQueries({ queryKey: ["admin", "users"] });
            qc.invalidateQueries({ queryKey: ["admin", "users", userId] });
            setReason("");
            onOpenChange(false);
        },
        onError: (err: Error) => {
            toast.error(err.message || "Action failed");
        },
    });

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>
                        {mode === "ban" ? "Ban user" : "Unban user"}
                    </DialogTitle>
                    <DialogDescription>
                        {mode === "ban" ? "Banning" : "Unbanning"}{" "}
                        <span className="font-mono">{userEmail}</span>. The reason
                        is written to the admin audit log and visible to other
                        admins.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-1.5">
                    <Label htmlFor="reason" className="text-xs font-medium text-muted-foreground">
                        Reason
                    </Label>
                    <Input
                        id="reason"
                        placeholder={
                            mode === "ban"
                                ? "e.g. repeated spam complaints, refused to honor unsubscribe"
                                : "e.g. resolved after appeal, false positive"
                        }
                        value={reason}
                        onChange={(e) => setReason(e.target.value)}
                        autoFocus
                    />
                </div>

                {mode === "ban" && (
                    <div>
                        <div className="text-xs font-medium text-muted-foreground">Scope</div>
                        <p className="mt-0.5 mb-2 text-xs text-subtle-foreground">
                            Which actions stop working while this ban is active.
                            At least one is required.
                        </p>
                        <div className="divide-y divide-border/70 overflow-hidden surface-lit rounded-xl border border-border bg-card">
                            <ScopeOption
                                label="Block login"
                                hint="user cannot authenticate"
                                checked={scopeLogin}
                                onChange={setScopeLogin}
                            />
                            <ScopeOption
                                label="Block workspace creation"
                                hint="user can stay logged in but cannot spin up a new org"
                                checked={scopeOrgCreate}
                                onChange={setScopeOrgCreate}
                            />
                            <ScopeOption
                                label="Block outbound send"
                                hint="campaign sending is paused while the ban is active"
                                checked={scopeSend}
                                onChange={setScopeSend}
                            />
                        </div>
                    </div>
                )}

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={() => {
                            if (reason.trim() === "") {
                                toast.error("Reason is required");
                                return;
                            }
                            if (
                                mode === "ban" &&
                                !scopeLogin &&
                                !scopeOrgCreate &&
                                !scopeSend
                            ) {
                                toast.error("Pick at least one scope");
                                return;
                            }
                            mutation.mutate();
                        }}
                        disabled={mutation.isPending}
                        variant={mode === "ban" ? "destructive" : "default"}
                    >
                        {mutation.isPending
                            ? "Working…"
                            : mode === "ban"
                            ? "Ban user"
                            : "Unban user"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

function ScopeOption({
    label,
    hint,
    checked,
    onChange,
}: {
    label: string;
    hint: string;
    checked: boolean;
    onChange: (v: boolean) => void;
}) {
    return (
        <label className="flex cursor-pointer items-start gap-2.5 px-3 py-2.5 transition-colors hover:bg-accent/50">
            <Checkbox
                checked={checked}
                onCheckedChange={(v) => onChange(v === true)}
                className="mt-0.5"
            />
            <span className="min-w-0 text-[13px]">
                <span className="font-medium text-foreground">{label}</span>
                <span className="block text-xs text-muted-foreground">{hint}</span>
            </span>
        </label>
    );
}
