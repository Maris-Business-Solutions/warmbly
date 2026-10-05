// Override editor for a single org. Each numeric field accepts:
//   - blank        → leave the field alone on submit (partial PUT)
//   - 0            → remove that column's override (revert to plan/hard cap)
//   - any positive → explicit cap that overrides the plan default
//
// We render plan default + current override + the effective number so
// the admin can see what's being enforced before changing anything.

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
import { updateOrganizationOverrides } from "@/lib/api/client/admin/organizations";
import type {
    AdminOrgDetail,
    OrganizationLimitOverrides,
    UpdateOrgOverridesRequest,
} from "@/lib/api/models/admin";

type FieldKey =
    | "max_email_accounts"
    | "max_campaigns"
    | "max_active_campaigns"
    | "max_team_members"
    | "max_contacts"
    | "daily_campaign_limit";

const FIELDS: { key: FieldKey; label: string; hint: string }[] = [
    { key: "max_email_accounts", label: "Mailboxes", hint: "Total connected mailboxes" },
    { key: "max_campaigns", label: "Campaigns (lifetime)", hint: "All campaigns ever created" },
    { key: "max_active_campaigns", label: "Active campaigns", hint: "Running at the same time" },
    { key: "max_team_members", label: "Team members", hint: "Seats in this workspace" },
    { key: "max_contacts", label: "Contacts", hint: "Stored recipient records" },
    { key: "daily_campaign_limit", label: "Daily sends", hint: "Campaign emails per day" },
];

type FormState = Record<FieldKey, string> & { notes: string };

function emptyForm(overrides: OrganizationLimitOverrides | null | undefined): FormState {
    const blank: FormState = {
        max_email_accounts: "",
        max_campaigns: "",
        max_active_campaigns: "",
        max_team_members: "",
        max_contacts: "",
        daily_campaign_limit: "",
        notes: "",
    };
    if (!overrides) return blank;
    return {
        max_email_accounts: String(overrides.max_email_accounts),
        max_campaigns: String(overrides.max_campaigns),
        max_active_campaigns: String(overrides.max_active_campaigns),
        max_team_members: String(overrides.max_team_members),
        max_contacts: String(overrides.max_contacts),
        daily_campaign_limit: String(overrides.daily_campaign_limit),
        notes: overrides.notes,
    };
}

export function OrganizationOverridesDialog({
    org,
    open,
    onOpenChange,
}: {
    org: AdminOrgDetail;
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [form, setForm] = useState<FormState>(() => emptyForm(org.overrides));

    // Reseed the form when the dialog reopens against a fresh org snapshot.
    useEffect(() => {
        if (open) setForm(emptyForm(org.overrides));
    }, [open, org.overrides]);

    const mutation = useMutation({
        mutationFn: (req: UpdateOrgOverridesRequest) =>
            updateOrganizationOverrides(org.id, req),
        onSuccess: () => {
            toast.success("Overrides saved");
            qc.invalidateQueries({ queryKey: ["admin", "organizations", org.id] });
            qc.invalidateQueries({ queryKey: ["admin", "organizations"] });
            onOpenChange(false);
        },
        onError: (err: Error) => {
            toast.error(err.message || "Failed to save overrides");
        },
    });

    function submit() {
        const req: UpdateOrgOverridesRequest = {};
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
        if (form.notes.trim() !== (org.overrides?.notes ?? "")) {
            req.notes = form.notes;
        }
        if (Object.keys(req).length === 0) {
            toast.error("Nothing changed");
            return;
        }
        mutation.mutate(req);
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle>Limit overrides</DialogTitle>
                    <DialogDescription>
                        Leave blank to keep the current value. <span className="font-medium text-foreground">0</span> removes
                        the override (back to plan default or product hard cap). Positive
                        numbers set an explicit ceiling that overrides everything else.
                    </DialogDescription>
                </DialogHeader>

                <div className="grid gap-4">
                    <div className="overflow-hidden rounded-lg border border-border">
                        <div className="grid h-9 grid-cols-[1fr_4rem_4.5rem_6rem] items-center gap-3 border-b border-border px-3 text-xs font-medium text-muted-foreground">
                            <span>Limit</span>
                            <span className="text-right">Plan</span>
                            <span className="text-right">Effective</span>
                            <span className="text-right">Override</span>
                        </div>
                        <div className="divide-y divide-border/70">
                            {FIELDS.map((f) => {
                                const plan = (org.limits ?? {}) as Record<string, number | null | undefined>;
                                const effective = (org.effective_limits ?? {}) as Record<string, number | null | undefined>;
                                return (
                                    <div
                                        key={f.key}
                                        className="grid grid-cols-[1fr_4rem_4.5rem_6rem] items-center gap-3 px-3 py-2 text-[13px]"
                                    >
                                        <div className="min-w-0">
                                            <Label htmlFor={f.key} className="text-[13px] font-medium text-foreground">
                                                {f.label}
                                            </Label>
                                            <div className="mt-0.5 truncate text-xs text-muted-foreground">
                                                {f.hint}
                                            </div>
                                        </div>
                                        <div className="text-right tabular-nums text-muted-foreground">
                                            {plan[f.key] != null ? plan[f.key] : "—"}
                                        </div>
                                        <div className="text-right font-medium tabular-nums text-foreground">
                                            {effective[f.key] != null ? effective[f.key] : "—"}
                                        </div>
                                        <Input
                                            id={f.key}
                                            inputMode="numeric"
                                            placeholder="—"
                                            value={form[f.key]}
                                            onChange={(e) =>
                                                setForm((s) => ({ ...s, [f.key]: e.target.value }))
                                            }
                                            className="h-7 w-24 text-right tabular-nums"
                                        />
                                    </div>
                                );
                            })}
                        </div>
                    </div>

                    <div className="space-y-1.5">
                        <Label htmlFor="notes" className="text-xs font-medium text-muted-foreground">
                            Notes
                        </Label>
                        <Input
                            id="notes"
                            placeholder="Reason for the change (visible to other admins)"
                            value={form.notes}
                            onChange={(e) =>
                                setForm((s) => ({ ...s, notes: e.target.value }))
                            }
                        />
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button onClick={submit} disabled={mutation.isPending}>
                        {mutation.isPending ? "Saving…" : "Save overrides"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
