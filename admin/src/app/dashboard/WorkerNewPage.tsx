// Add a machine to the fleet.
//
// There is no form here worth filling in, because there is nothing to
// configure: you issue a token, run one command on a machine you already own,
// and it appears. Everything it needs — event bus, cache, keys, the version to
// run — is handed to it by the control plane at join time, so the only two
// choices are what the machine does (worker or consumer) and, optionally,
// where it is.

import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft, Check, Copy, KeyRound, Server, Wrench } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { API_URL } from "@/lib/env";
import { issueJoinToken, type NodeRole } from "@/lib/api/client/admin/fleetNodes";

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
            size="sm"
            variant="outline"
            onClick={async () => {
                await navigator.clipboard.writeText(value);
                setCopied(true);
                toast.success(`${label} copied`);
                window.setTimeout(() => setCopied(false), 1500);
            }}
        >
            {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
            {copied ? "Copied" : "Copy"}
        </Button>
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
        <div className="space-y-4">
            <PageHeader
                title="Add a machine"
                description="Run one command on any machine you own. Nothing connects back to it."
            >
                <Button asChild size="sm" variant="outline">
                    <Link to="/workers">
                        <ArrowLeft className="size-4" />
                        Fleet
                    </Link>
                </Button>
            </PageHeader>

            <Card>
                <CardHeader>
                    <CardTitle className="text-base">What should it do?</CardTitle>
                    <CardDescription>
                        Both roles enrol, report themselves and stay on the version you choose.
                        The difference is only what work they pick up.
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="grid gap-2 sm:grid-cols-2">
                        <button
                            type="button"
                            onClick={() => setRole("worker")}
                            className={`rounded-md border p-3 text-left transition ${
                                role === "worker"
                                    ? "border-[var(--admin-accent-strong)] bg-[var(--admin-accent-weak)]"
                                    : "hover:bg-muted/50"
                            }`}
                        >
                            <span className="flex items-center gap-2 text-sm font-medium">
                                <Server className="size-4" />
                                Worker
                            </span>
                            <span className="mt-1 block text-xs text-muted-foreground">
                                Connects to customer mailboxes to send and sync. Add these when
                                capacity runs low.
                            </span>
                        </button>
                        <button
                            type="button"
                            onClick={() => setRole("consumer")}
                            className={`rounded-md border p-3 text-left transition ${
                                role === "consumer"
                                    ? "border-[var(--admin-accent-strong)] bg-[var(--admin-accent-weak)]"
                                    : "hover:bg-muted/50"
                            }`}
                        >
                            <span className="flex items-center gap-2 text-sm font-medium">
                                <Wrench className="size-4" />
                                Consumer
                            </span>
                            <span className="mt-1 block text-xs text-muted-foreground">
                                Processes events and keeps platform state current. They share work
                                automatically, so more of them just works.
                            </span>
                        </button>
                    </div>

                    {role === "worker" && (
                        <div className="space-y-1.5">
                            <Label htmlFor="region">Region (optional)</Label>
                            <Input
                                id="region"
                                value={region}
                                onChange={(e) => setRegion(e.target.value)}
                                placeholder="eu-central"
                            />
                            <p className="text-xs text-muted-foreground">
                                Where this machine egresses from. Placement prefers a worker near
                                where a mailbox&apos;s provider expects sign-ins, which means fewer
                                security challenges. Leave it blank and it scores neutral.
                            </p>
                        </div>
                    )}
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle className="text-base">Run this on the machine</CardTitle>
                    <CardDescription>
                        Needs Docker, systemd and root. The machine must be able to reach this
                        instance; nothing needs to reach it.
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                    {!token && (
                        <Button size="sm" onClick={() => issue.mutate()} disabled={issue.isPending}>
                            <KeyRound className="size-4" />
                            {issue.isPending ? "Issuing…" : "Issue a join token"}
                        </Button>
                    )}

                    <div className="relative">
                        <pre
                            className="overflow-x-auto rounded-md border bg-muted/40 p-3 pr-24 font-mono text-[12px] leading-relaxed"
                            data-ph-mask=""
                        >
                            {command}
                        </pre>
                        <div className="absolute right-2 top-2">
                            <CopyButton value={command} label="Command" />
                        </div>
                    </div>

                    {token ? (
                        <p className="text-xs text-amber-700">
                            This token is shown once and is not recoverable. It joins any number of
                            machines until {expiresAt ? new Date(expiresAt).toLocaleString() : "it expires"}.
                            Issuing another one revokes it; machines that already joined are unaffected.
                        </p>
                    ) : (
                        <p className="text-xs text-muted-foreground">
                            Issue a token to fill in the command. One token can add as many
                            machines as you like for seven days, or until you replace it.
                        </p>
                    )}
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle className="text-base">Then what</CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 text-[13px] text-muted-foreground">
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
                </CardContent>
            </Card>
        </div>
    );
}
