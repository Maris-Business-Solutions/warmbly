// Shared error panel for failed queries. Surfaces the WHOLE error — the
// human message plus the backend's machine-readable code, request id, and HTTP
// status — so a broken tab tells you exactly what went wrong (and gives you a
// request id to grep the logs) instead of a generic "failed to load".

import { AlertTriangle, RotateCw } from "lucide-react";
import { APIError, SessionExpiredError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface Props {
    error: unknown;
    title?: string;
    onRetry?: () => void;
    className?: string;
}

export function ErrorState({ error, title = "Couldn’t load this", onRetry, className }: Props) {
    const isSession = error instanceof SessionExpiredError;
    const api = error instanceof APIError ? error : null;

    const message = isSession
        ? "Your session expired — sign in again."
        : api?.message || (error instanceof Error ? error.message : "An unexpected error occurred.");

    const meta = [
        api?.status ? `HTTP ${api.status}` : null,
        api?.code ? `code ${api.code}` : null,
        api?.requestId ? `request ${api.requestId}` : null,
    ].filter(Boolean) as string[];

    return (
        <div className={cn("rounded-lg border border-red-500/20 bg-red-500/[0.06] p-4", className)}>
            <div className="flex items-start gap-3">
                <AlertTriangle className="mt-0.5 size-4 shrink-0 text-red-600 dark:text-red-400" />
                <div className="min-w-0 flex-1">
                    <div className="text-[13px] font-medium text-foreground">{title}</div>
                    <p className="mt-0.5 text-[13px] leading-relaxed text-muted-foreground break-words">{message}</p>
                    {meta.length > 0 && (
                        <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 font-mono text-[11px] text-subtle-foreground select-text">
                            {meta.map((m, i) => (
                                <span key={m} className="inline-flex items-center gap-2">
                                    {i > 0 && <span className="opacity-50">·</span>}
                                    {m}
                                </span>
                            ))}
                        </div>
                    )}
                    {onRetry && (
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={onRetry}
                            className="mt-3"
                        >
                            <RotateCw className="size-3.5" />
                            Retry
                        </Button>
                    )}
                </div>
            </div>
        </div>
    );
}
