// Platform mail transport: what it is, whether it dials, and a way to send a
// real message through it.
//
// Platform mail is not a backing service that merely degrades. Login codes,
// password resets and team invitations all go through it, so a relay that
// cannot authenticate locks everyone out, and the only symptom used to be a
// generic 500 on the login screen. Mattermost, Gitea, Vaultwarden and Zulip all
// ship an equivalent, and it is consistently their most deflected support
// ticket.

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Mail, RefreshCw, Send, XCircle } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Callout, Panel, StatusBadge } from "@/components/ui/kit";
import { getMailStatus, sendTestEmail } from "@/lib/api/client/admin/system";
import { APIError } from "@/lib/api/client";
import { cn } from "@/lib/utils";

export function MailStatusCard() {
    const [to, setTo] = useState("");

    const statusQ = useQuery({
        queryKey: ["admin", "mail", "status"],
        queryFn: getMailStatus,
        retry: false,
    });

    const testM = useMutation({
        mutationFn: (address: string) => sendTestEmail(address),
        onSuccess: (result) => {
            if (!result.sent) {
                toast.error(result.error || "The send failed.");
                return;
            }
            toast.success(result.note || `Sent through the ${result.transport} transport.`);
        },
        onError: (err) => toast.error(err instanceof APIError ? err.message : "The send failed."),
    });

    const status = statusQ.data;

    return (
        <Panel
            className={cn(status && !status.healthy && "border-red-500/30")}
            title={
                <span className="flex items-center gap-2">
                    <Mail className="size-4 text-subtle-foreground" />
                    Platform mail
                </span>
            }
            description={status ? `${status.transport} transport` : undefined}
            actions={
                <>
                    {status && (
                        <StatusBadge tone={status.healthy ? "success" : "danger"} dot>
                            {status.healthy ? "Reachable" : "Failing"}
                        </StatusBadge>
                    )}
                    <Button
                        size="icon-sm"
                        variant="ghost"
                        aria-label="Refresh mail status"
                        onClick={() => statusQ.refetch()}
                        disabled={statusQ.isFetching}
                    >
                        <RefreshCw className={cn("size-3.5", statusQ.isFetching && "animate-spin")} />
                    </Button>
                </>
            }
            bodyClassName="space-y-3"
        >
            {statusQ.isLoading && <Skeleton className="h-16 w-full" />}

            {status && (
                <>
                    <div className="flex items-start gap-2 text-[13px]">
                        {status.healthy ? (
                            <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                        ) : (
                            <XCircle className="mt-0.5 size-4 shrink-0 text-red-600 dark:text-red-400" />
                        )}
                        <span className="leading-relaxed text-muted-foreground">{status.detail}</span>
                    </div>

                    {status.error && (
                        <pre className="overflow-x-auto rounded-md border border-red-500/20 bg-red-500/[0.06] p-2.5 font-mono text-[11.5px] leading-relaxed whitespace-pre-wrap break-words text-red-700 dark:text-red-400">
                            {status.error}
                        </pre>
                    )}

                    {!status.delivers && (
                        <Callout tone="warning" icon={AlertTriangle}>
                            Nothing is being delivered. Every platform email, including login codes, is
                            written to the backend logs. Set MAIL_TRANSPORT=smtp and the SMTP_ variables
                            before anyone else relies on this instance.
                        </Callout>
                    )}

                    <div className="border-t border-border pt-3">
                        <div className="mb-1.5 text-xs font-medium text-muted-foreground">Send a test message</div>
                        <div className="flex items-center gap-2">
                            <Input
                                type="email"
                                value={to}
                                onChange={(e) => setTo(e.target.value)}
                                placeholder="you@example.com"
                                aria-label="Test recipient"
                            />
                            <Button
                                variant="outline"
                                onClick={() => testM.mutate(to)}
                                disabled={!to || testM.isPending}
                            >
                                <Send />
                                {testM.isPending ? "Sending..." : "Send test"}
                            </Button>
                        </div>
                    </div>
                </>
            )}
        </Panel>
    );
}
