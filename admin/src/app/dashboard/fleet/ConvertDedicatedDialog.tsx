// Reserve a worker for one workspace, so its mailboxes authenticate to their
// providers from an address nobody else uses.
//
// Reserving only writes the binding. Mailboxes already on the worker are not
// evicted here: the rotation loop moves other tenants off on its own schedule,
// which is why there is no drain step to fill in.

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
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
import { convertWorkerToDedicated } from "@/lib/api/client/admin/fleet";
import { listFleetNodes, nodeState, type FleetNode } from "@/lib/api/client/admin/fleetNodes";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";
import { OrgPicker, type PickedOrg } from "./OrgPicker";

const FIELD_LABEL = "text-xs font-medium text-muted-foreground";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function workerLabel(w: FleetNode): string {
    const region = w.region ? ` · ${w.region}` : "";
    return `${w.name || w.id.slice(0, 8)}${region} · ${nodeState(w)} · ${(w.mailbox_count ?? 0)} mailbox${(w.mailbox_count ?? 0) === 1 ? "" : "es"}`;
}

export function ConvertDedicatedDialog({
    open,
    onOpenChange,
}: {
    open: boolean;
    onOpenChange: (v: boolean) => void;
}) {
    const qc = useQueryClient();
    const [workerId, setWorkerId] = useState("");
    const [org, setOrg] = useState<PickedOrg | null>(null);
    const [subscriptionId, setSubscriptionId] = useState("");

    const workersQ = useQuery({
        queryKey: ["admin", "workers", "managed"],
        queryFn: () => listFleetNodes("worker"),
        enabled: open,
        staleTime: 30_000,
    });
    const workers = workersQ.data?.data ?? [];
    // Any worker can be reserved: there is no category to check.
    const shared = workers;
    const subOk = UUID_RE.test(subscriptionId.trim());
    const canSubmit = !!workerId && !!org && subOk;

    const mutation = useMutation({
        mutationFn: () =>
            convertWorkerToDedicated(workerId, {
                organization_id: org!.id,
                subscription_id: subscriptionId.trim(),
            }),
        onSuccess: (res) => {
            toast.success(
                res.new_reservation
                    ? `Worker reserved for ${org?.name}. Other tenants drift off it on the rotation loop.`
                    : "That workspace already had this worker reserved.",
            );
            qc.invalidateQueries({ queryKey: ["admin", "workers"] });
            qc.invalidateQueries({ queryKey: ["admin", "fleet"] });
            reset();
            onOpenChange(false);
        },
        onError: (e: Error) => toast.error(e.message || "Conversion failed"),
    });

    function reset() {
        setWorkerId("");
        setOrg(null);
        setSubscriptionId("");
    }

    return (
        <Dialog
            open={open}
            onOpenChange={(v) => {
                if (!v && mutation.isPending) return;
                if (!v) reset();
                onOpenChange(v);
            }}
        >
            <DialogContent
                onEscapeKeyDown={(e) => {
                    // A picker popover owns Escape while it is open.
                    if (document.querySelector("[data-floating]")) e.preventDefault();
                }}
            >
                <DialogHeader>
                    <DialogTitle>Reserve a worker</DialogTitle>
                    <DialogDescription>
                        This workspace&apos;s mailboxes will sign in from an address no other
                        tenant uses. Placement prefers the reserved worker for them, and moves
                        other tenants off it over the following passes.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-1.5">
                        <Label className={FIELD_LABEL}>Worker</Label>
                        <Select value={workerId || undefined} onValueChange={setWorkerId}>
                            <SelectTrigger className="h-8 w-full text-[13px]">
                                <SelectValue placeholder={workersQ.isLoading ? "Loading workers…" : "Pick a worker"} />
                            </SelectTrigger>
                            <SelectContent>
                                {shared.length === 0 && (
                                    <div className="px-2 py-1.5 text-xs text-muted-foreground">No workers.</div>
                                )}
                                {shared.map((w) => (
                                    <SelectItem key={w.id} value={w.id} className="text-[12.5px]">
                                        {workerLabel(w)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="space-y-1.5">
                        <Label className={FIELD_LABEL}>Workspace</Label>
                        <OrgPicker value={org} onChange={setOrg} />
                    </div>

                    <div className="space-y-1.5">
                        <Label htmlFor="sub-id" className={FIELD_LABEL}>
                            Subscription id
                        </Label>
                        <Input
                            id="sub-id"
                            value={subscriptionId}
                            onChange={(e) => setSubscriptionId(e.target.value)}
                            placeholder="00000000-0000-0000-0000-000000000000"
                            aria-invalid={!!subscriptionId && !subOk}
                            className="h-8 font-mono text-[12.5px]"
                        />
                        <p className="text-xs leading-relaxed text-muted-foreground">
                            The workspace's subscription row (a UUID). The organization page shows only the plan and
                            status, so read the id from the <code>subscriptions</code> table for this workspace, or from
                            the Stripe subscription's metadata.
                            {subscriptionId && !subOk && (
                                <span className={cn("ml-1 font-medium", TONE_TEXT.danger)}>Not a UUID.</span>
                            )}
                        </p>
                    </div>

                    <p className="border-t border-border pt-3 text-xs leading-relaxed text-muted-foreground">
                        Mailboxes already on this worker are not evicted here. The rotation loop
                        moves other tenants off it on its own schedule, so the reservation
                        becomes exclusive without re-authenticating every mailbox at once.
                    </p>
                </div>

                <DialogFooter>
                    <Button
                        variant="outline"
                        onClick={() => {
                            reset();
                            onOpenChange(false);
                        }}
                        disabled={mutation.isPending}
                    >
                        Cancel
                    </Button>
                    <Button onClick={() => mutation.mutate()} disabled={!canSubmit || mutation.isPending}>
                        {mutation.isPending ? "Reserving…" : "Reserve worker"}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
