// User rate-limit override editor. user_rate_limits has no null state, so a
// blank field means "leave it alone" and clearing means typing the default back.

import { useEffect, useState } from "react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { updateUserRateLimits } from "@/lib/api/client/admin/users";
import type {
    AdminUserRateLimits,
    UpdateUserRateLimitsRequest,
} from "@/lib/api/models/admin";

type FieldKey =
    | "limit_read_pm"
    | "limit_write_pm"
    | "limit_bulk_pm"
    | "limit_unibox_pm"
    | "limit_analytics_pm"
    | "limit_api_calls_daily"
    | "limit_bulk_ops_daily"
    | "max_connections"
    | "limit_ws_message_pm"
    | "limit_ws_join_pm"
    | "limit_ws_event_pm";

const FIELDS: { key: FieldKey; label: string; hint: string }[] = [
    { key: "limit_read_pm", label: "Reads / min", hint: "GET-style API calls per minute" },
    { key: "limit_write_pm", label: "Writes / min", hint: "Mutating API calls per minute" },
    { key: "limit_bulk_pm", label: "Bulk / min", hint: "Bulk import and export calls per minute" },
    { key: "limit_unibox_pm", label: "Unibox / min", hint: "Unified inbox calls per minute" },
    { key: "limit_analytics_pm", label: "Analytics / min", hint: "Analytics calls per minute" },
    { key: "limit_api_calls_daily", label: "API calls / day", hint: "Total API calls per day" },
    { key: "limit_bulk_ops_daily", label: "Bulk ops / day", hint: "Bulk operations per day" },
    { key: "max_connections", label: "Max connections", hint: "Concurrent websocket sessions" },
    { key: "limit_ws_message_pm", label: "WS messages / min", hint: "Realtime messages per minute" },
    { key: "limit_ws_join_pm", label: "WS joins / min", hint: "Realtime channel joins per minute" },
    { key: "limit_ws_event_pm", label: "WS events / min", hint: "Realtime broadcast events per minute" },
];

export function UserRateLimitsDialog({
    userId,
    userEmail,
    current,
    open,
    onOpenChange,
}: {
    userId: string;
    userEmail: string;
    current: AdminUserRateLimits | null | undefined;
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [form, setForm] = useState<Record<FieldKey, string>>(blankForm);

    // Reset on open only: refetching `current` mid-edit must not wipe the form.
    useEffect(() => {
        if (open) setForm(blankForm());
    }, [open]);

    const mutation = useMutation({
        mutationFn: (req: UpdateUserRateLimitsRequest) => updateUserRateLimits(userId, req),
        onSuccess: () => {
            toast.success("Rate limits updated");
            qc.invalidateQueries({ queryKey: ["admin", "users", userId] });
            qc.invalidateQueries({ queryKey: ["admin", "users", userId, "rate-limits"] });
            onOpenChange(false);
        },
        onError: (err: Error) => {
            toast.error(err.message || "Failed to update rate limits");
        },
    });

    function submit() {
        const req: UpdateUserRateLimitsRequest = {};
        for (const f of FIELDS) {
            const raw = form[f.key].trim();
            if (raw === "") continue;
            const n = Number(raw);
            if (!Number.isInteger(n) || n < 0) {
                toast.error(`${f.label}: must be a non-negative integer`);
                return;
            }
            req[f.key] = n;
        }
        if (Object.keys(req).length === 0) {
            toast.error("Nothing changed");
            return;
        }
        mutation.mutate(req);
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-xl">
                <DialogHeader>
                    <DialogTitle>Rate limit overrides</DialogTitle>
                    <DialogDescription>
                        Per-user overrides for{" "}
                        <span className="font-mono">{userEmail}</span>. Leave a
                        field blank to keep its current value. There is no null
                        state here, so <strong>0</strong> means zero, not
                        "inherit".
                    </DialogDescription>
                </DialogHeader>

                <div className="-mx-1 max-h-[60vh] overflow-y-auto px-1">
                    <div className="grid grid-cols-[1fr_auto_auto] items-center gap-3 border-b border-border pb-1.5 text-xs font-medium text-muted-foreground">
                        <span>Limit</span>
                        <span className="text-right">Current</span>
                        <span className="w-28 text-right">New value</span>
                    </div>
                    {FIELDS.map((f) => {
                        const cur = current?.[f.key];
                        return (
                            <div
                                key={f.key}
                                className="grid grid-cols-[1fr_auto_auto] items-center gap-3 border-b border-border/70 py-2 last:border-b-0"
                            >
                                <div className="min-w-0">
                                    <Label htmlFor={f.key} className="text-[13px] font-medium text-foreground">
                                        {f.label}
                                    </Label>
                                    <div className="mt-0.5 truncate text-xs text-muted-foreground">
                                        {f.hint}
                                    </div>
                                </div>
                                <div className="text-right text-[13px] tabular-nums text-muted-foreground">
                                    {cur != null ? cur.toLocaleString() : "—"}
                                </div>
                                <Input
                                    id={f.key}
                                    inputMode="numeric"
                                    placeholder="—"
                                    value={form[f.key]}
                                    onChange={(e) =>
                                        setForm((s) => ({ ...s, [f.key]: e.target.value }))
                                    }
                                    className="h-7 w-28 text-right tabular-nums"
                                />
                            </div>
                        );
                    })}
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button
                        onClick={submit}
                        disabled={mutation.isPending}
                    >
                        {mutation.isPending ? "Saving…" : "Save"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Blank on purpose: the current value is shown beside the input, not in it.
function blankForm(): Record<FieldKey, string> {
    return FIELDS.reduce(
        (acc, f) => ({ ...acc, [f.key]: "" }),
        {} as Record<FieldKey, string>,
    );
}
