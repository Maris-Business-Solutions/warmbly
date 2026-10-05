// Pin a workspace's posture to an operator's decision.
//
// The pin outranks the score and survives every later detector write, so
// clearing a workspace by review is not undone by the evidence still on file.
// The reason is shown to the workspace itself, which is why the field says so.

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
import { TONE_DOT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { setOrganizationRiskOverride } from "@/lib/api/client/admin/organizations";
import type { OrgRisk, OrgRiskState } from "@/lib/api/models/admin";

const BANDS: { state: OrgRiskState; effect: string; dot: string }[] = [
    { state: "trusted", effect: "Nothing is restricted", dot: TONE_DOT.success },
    { state: "watch", effect: "Nothing the workspace can feel", dot: TONE_DOT.warning },
    { state: "restricted", effect: "Quarter volume, free warmup pool", dot: TONE_DOT.orange },
    { state: "suspended", effect: "Sending stops entirely", dot: TONE_DOT.danger },
];

export function OrganizationRiskDialog({
    orgId,
    risk,
    open,
    onOpenChange,
}: {
    orgId: string;
    risk: OrgRisk;
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [state, setState] = useState<OrgRiskState>(risk.state);
    const [reason, setReason] = useState(risk.override?.reason ?? "");

    // Reseed against a fresh record each time it opens.
    useEffect(() => {
        if (open) {
            setState(risk.state);
            setReason(risk.override?.reason ?? "");
        }
    }, [open, risk.state, risk.override?.reason]);

    const mutation = useMutation({
        mutationFn: () => setOrganizationRiskOverride(orgId, { state, reason: reason.trim() }),
        onSuccess: (updated) => {
            qc.setQueryData(["admin", "organizations", orgId, "risk"], updated);
            qc.invalidateQueries({ queryKey: ["admin", "organizations", orgId] });
            qc.invalidateQueries({ queryKey: ["admin", "organizations"] });
            toast.success(`Posture pinned to ${updated.state}`);
            onOpenChange(false);
        },
        onError: (e: Error) => toast.error(e.message || "Failed to set the posture"),
    });

    function submit() {
        if (reason.trim() === "") {
            toast.error("A reason is required");
            return;
        }
        mutation.mutate();
    }

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>Set workspace posture</DialogTitle>
                    <DialogDescription>
                        This pins the band. Detectors keep scoring the evidence, but the
                        posture stays where you put it until the override is lifted. The
                        score right now is <span className="font-medium tabular-nums text-foreground">{risk.score}</span>.
                    </DialogDescription>
                </DialogHeader>

                <div className="grid gap-4">
                    <div
                        role="radiogroup"
                        aria-label="Posture"
                        className="overflow-hidden rounded-lg border border-border divide-y divide-border"
                    >
                        {BANDS.map((b) => {
                            const active = state === b.state;
                            return (
                                <label
                                    key={b.state}
                                    className={cn(
                                        "flex h-10 cursor-pointer items-center gap-3 px-3 text-[13px] transition-colors has-[:focus-visible]:bg-accent/60",
                                        active ? "bg-[var(--admin-accent-weak)]" : "hover:bg-accent/50",
                                    )}
                                >
                                    <input
                                        type="radio"
                                        name="risk-state"
                                        value={b.state}
                                        checked={active}
                                        onChange={() => setState(b.state)}
                                        className="sr-only"
                                    />
                                    <span
                                        aria-hidden
                                        className={cn(
                                            "grid size-3.5 shrink-0 place-items-center rounded-full border transition-colors",
                                            active ? "border-[var(--admin-accent)]" : "border-border-strong",
                                        )}
                                    >
                                        {active && <span className="size-1.5 rounded-full bg-[var(--admin-accent)]" />}
                                    </span>
                                    <span className="inline-flex w-24 items-center gap-1.5 font-medium text-foreground">
                                        <span className={cn("size-1.5 rounded-full", b.dot)} />
                                        {b.state}
                                    </span>
                                    <span className="truncate text-xs text-muted-foreground">{b.effect}</span>
                                </label>
                            );
                        })}
                    </div>

                    <div className="space-y-1.5">
                        <Label htmlFor="risk-reason" className="text-xs font-medium text-muted-foreground">
                            Reason
                        </Label>
                        <Input
                            id="risk-reason"
                            placeholder="Why this workspace is where you are putting it"
                            value={reason}
                            onChange={(e) => setReason(e.target.value)}
                        />
                        <p className="text-xs leading-relaxed text-muted-foreground">
                            Shown to the workspace in its dashboard banner, so write it for
                            them. Internal notes belong in the audit trail, not here.
                        </p>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        Cancel
                    </Button>
                    <Button onClick={submit} disabled={mutation.isPending}>
                        {mutation.isPending ? "Saving…" : "Pin posture"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
