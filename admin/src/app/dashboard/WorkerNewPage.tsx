// Add a machine to the fleet.
//
// There is no form here worth filling in, because there is nothing to
// configure: you issue a token, run one command on a machine you already own,
// and it appears. Everything it needs (event bus, cache, keys, the version to
// run) is handed to it by the control plane at join time, so the only two
// choices are what the machine does (worker or consumer) and, optionally,
// where it is.

import { useMemo, useState, type ReactNode } from "react";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { Check, Copy, KeyRound, Server, TriangleAlert, Wrench, type LucideIcon } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Callout } from "@/components/ui/kit";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { API_URL } from "@/lib/env";
import { issueJoinToken, type NodeRole } from "@/lib/api/client/admin/fleetNodes";
import { cn } from "@/lib/utils";

// The instance a node should point at. API_URL carries the /api/v1 prefix the
// client uses; the join command wants the bare origin.
function instanceOrigin(): string {
    try {
        return new URL(API_URL, window.location.origin).origin;
    } catch {
        return window.location.origin;
    }
}

function CopyButton({ value, label }: { value: string; label: string }) {
    const [copied, setCopied] = useState(false);
    return (
        <Button
            size="xs"
            variant="outline"
            className="bg-card"
            onClick={async () => {
                await navigator.clipboard.writeText(value);
                setCopied(true);
                toast.success(`${label} copied`);
                window.setTimeout(() => setCopied(false), 1500);
            }}
        >
            {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
            {copied ? "Copied" : "Copy"}
        </Button>
    );
}

type StepState = "done" | "current" | "upcoming";

// One numbered step on the setup rail; the connector runs down to the next.
function Step({
    n,
    state,
    title,
    description,
    last,
    children,
}: {
    n: number;
    state: StepState;
    title: ReactNode;
    description?: ReactNode;
    last?: boolean;
    children: ReactNode;
}) {
    return (
        <li className="relative flex gap-4">
            {!last && <span className="absolute top-7 bottom-0 left-[11px] w-px bg-border" aria-hidden />}
            <span
                className={cn(
                    "relative z-10 mt-0.5 grid size-6 shrink-0 place-items-center rounded-full border text-xs font-medium tabular-nums",
                    state === "done" && "border-transparent bg-[var(--admin-accent)] text-[var(--admin-accent-foreground)]",
                    state === "current" &&
                        "border-[var(--admin-accent)] bg-[var(--admin-accent-weak)] text-[var(--admin-accent-strong)]",
                    state === "upcoming" && "border-border-strong bg-card text-muted-foreground",
                )}
            >
                {state === "done" ? <Check className="size-3.5" /> : n}
            </span>
            <div className={cn("min-w-0 flex-1", !last && "pb-10")}>
                <h2 className="text-[13px] leading-7 font-semibold text-foreground">{title}</h2>
                {description && (
                    <p className="max-w-2xl text-[12.5px] leading-relaxed text-muted-foreground">{description}</p>
                )}
                <div className="mt-3">{children}</div>
            </div>
        </li>
    );
}

function RoleOption({
    active,
    onClick,
    icon: Icon,
    title,
    children,
}: {
    active: boolean;
    onClick: () => void;
    icon: LucideIcon;
    title: string;
    children: ReactNode;
}) {
    return (
        <button
            type="button"
            role="radio"
            aria-checked={active}
            onClick={onClick}
            className={cn(
                "flex gap-3 rounded-lg border p-3 text-left transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                active
                    ? "border-[var(--admin-accent)] bg-[var(--admin-accent-weak)]"
                    : "border-border bg-card hover:border-border-strong hover:bg-accent/50",
            )}
        >
            <span
                className={cn(
                    "grid size-7 shrink-0 place-items-center rounded-md border",
                    active
                        ? "border-transparent bg-[var(--admin-accent-soft)] text-[var(--admin-accent-strong)]"
                        : "border-border bg-muted/50 text-subtle-foreground",
                )}
            >
                <Icon className="size-3.5" />
            </span>
            <span className="min-w-0">
                <span className="block text-[13px] font-medium text-foreground">{title}</span>
                <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">{children}</span>
            </span>
        </button>
    );
}

export default function WorkerNewPage() {
    const [role, setRole] = useState<NodeRole>("worker");
    const [region, setRegion] = useState("");
    const [token, setToken] = useState<string | null>(null);
    const [expiresAt, setExpiresAt] = useState<string | null>(null);

    const origin = instanceOrigin();

    const issue = useMutation({
        mutationFn: issueJoinToken,
        onSuccess: (res) => {
            setToken(res.token);
            setExpiresAt(res.expires_at);
            toast.success("Join token issued");
        },
        onError: (e: Error) => toast.error(e.message || "Could not issue a token"),
    });

    const command = useMemo(() => {
        const t = token ?? "<join-token>";
        const parts = [
            `curl -fsSL ${origin}/join.sh | sh -s -- \\`,
            `  --url ${origin} \\`,
            `  --token ${t} \\`,
            `  --role ${role}`,
        ];
        if (region.trim()) parts[parts.length - 1] += ` \\`;
        if (region.trim()) parts.push(`  --region ${region.trim()}`);
        return parts.join("\n");
    }, [origin, token, role, region]);

    return (
        <div>
            <PageHeader
                breadcrumbs={[{ label: "Workers", to: "/workers" }]}
                title="New worker"
                description="Run one command on any machine you own. Nothing connects back to it."
            />

            <ol className="max-w-3xl">
                <Step
                    n={1}
                    state="done"
                    title="What should it do?"
                    description="Both roles enrol, report themselves and stay on the version you choose. The difference is only what work they pick up."
                >
                    <div role="radiogroup" aria-label="Role" className="grid gap-2 sm:grid-cols-2">
                        <RoleOption active={role === "worker"} onClick={() => setRole("worker")} icon={Server} title="Worker">
                            Connects to customer mailboxes to send and sync. Add these when capacity runs low.
                        </RoleOption>
                        <RoleOption
                            active={role === "consumer"}
                            onClick={() => setRole("consumer")}
                            icon={Wrench}
                            title="Consumer"
                        >
                            Processes events and keeps platform state current. They share work automatically, so
                            more of them just works.
                        </RoleOption>
                    </div>

                    {role === "worker" && (
                        <div className="mt-4 max-w-sm space-y-1.5">
                            <Label htmlFor="region" className="text-xs font-medium text-muted-foreground">
                                Region (optional)
                            </Label>
                            <Input
                                id="region"
                                value={region}
                                onChange={(e) => setRegion(e.target.value)}
                                placeholder="eu-central"
                                className="font-mono text-[12.5px]"
                            />
                            <p className="text-xs leading-relaxed text-muted-foreground">
                                Where this machine egresses from. Placement prefers a worker near
                                where a mailbox&apos;s provider expects sign-ins, which means fewer
                                security challenges. Leave it blank and it scores neutral.
                            </p>
                        </div>
                    )}
                </Step>

                <Step
                    n={2}
                    state={token ? "done" : "current"}
                    title="Run this on the machine"
                    description="Needs Docker, systemd and root. The machine must be able to reach this instance; nothing needs to reach it."
                >
                    <div className="space-y-3">
                        {!token && (
                            <Button size="sm" onClick={() => issue.mutate()} disabled={issue.isPending}>
                                <KeyRound className="size-3.5" />
                                {issue.isPending ? "Issuing…" : "Issue a join token"}
                            </Button>
                        )}

                        <div className="relative overflow-hidden rounded-lg border border-border bg-muted/40">
                            <div className="flex h-9 items-center justify-between border-b border-border pr-1.5 pl-3 text-xs text-muted-foreground">
                                <span>Shell</span>
                                <CopyButton value={command} label="Command" />
                            </div>
                            <pre
                                className={cn(
                                    "overflow-x-auto p-3 font-mono text-[12px] leading-relaxed text-foreground",
                                    !token && "text-muted-foreground",
                                )}
                                data-ph-mask=""
                            >
                                {command}
                            </pre>
                        </div>

                        {token ? (
                            <Callout tone="warning" icon={TriangleAlert}>
                                This token is shown once and is not recoverable. It joins any number of
                                machines until {expiresAt ? new Date(expiresAt).toLocaleString() : "it expires"}.
                                Issuing another one revokes it; machines that already joined are unaffected.
                            </Callout>
                        ) : (
                            <p className="text-xs leading-relaxed text-muted-foreground">
                                Issue a token to fill in the command. One token can add as many
                                machines as you like for seven days, or until you replace it.
                            </p>
                        )}
                    </div>
                </Step>

                <Step n={3} state={token ? "current" : "upcoming"} title="Then what" last>
                    <div className="max-w-2xl space-y-2 text-[13px] leading-relaxed text-muted-foreground">
                        <p>
                            The machine appears in the fleet within a minute or two. A worker starts
                            taking mailboxes on its own; you never assign them by hand.
                        </p>
                        <p>
                            It also keeps itself on whatever version the fleet is set to, so there is
                            nothing to do when a release lands. Set that under Fleet.
                        </p>
                        <p>
                            Re-running the same command on the same machine re-joins it under the same
                            identity, keeping its history and its mailboxes.
                        </p>
                    </div>
                </Step>
            </ol>
        </div>
    );
}
