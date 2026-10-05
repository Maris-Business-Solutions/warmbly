// Setup and health, services: live probes against the platform's backing
// services. The backend runs them on each request, so every fetch is a real
// round-trip to postgres, redis, the event bus and friends, not a cached view.

import { useQuery } from "@tanstack/react-query";
import { CheckCircle2, RefreshCw, XCircle } from "lucide-react";
import { ErrorState } from "@/components/ErrorState";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Callout, StatusBadge, StatusDot } from "@/components/ui/kit";
import { cn } from "@/lib/utils";
import { MailStatusCard } from "../MailStatusCard";
import { getSystemStatus, type SystemComponentStatus } from "@/lib/api/client/admin/system";

export const SYSTEM_STATUS_KEY = ["admin", "system", "status"] as const;

// What breaks when each known component is down. Unknown names fall back
// to a generic line so new probes render without a frontend change.
const COMPONENT_BLURBS: Record<string, string> = {
    postgres: "Primary datastore. Everything depends on it.",
    redis: "Caching, rate limits, and the realtime event bridge in dev.",
    kafka: "Worker command and result transport. Nothing sends without it.",
    nats: "Worker command and result transport. Nothing sends without it.",
    "schema-registry": "Tracking-event encoding. Open and click events cannot serialize without it.",
    realtime: "Live dashboard updates (websockets).",
    tracking: "Open and click tracking ingestion.",
};

const GENERIC_BLURB = "Backing service probed by the backend health check.";

// "schema-registry" -> "Schema registry".
function titleCase(name: string): string {
    const spaced = name.replace(/[-_]+/g, " ").trim();
    if (!spaced) return name;
    return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

export function ServicesTab() {
    // Probes have no realtime event, so this is a deliberate poll; the query
    // only lives while the tab is mounted.
    const statusQ = useQuery({
        queryKey: SYSTEM_STATUS_KEY,
        queryFn: getSystemStatus,
        refetchInterval: 15_000,
        retry: false,
    });

    const components = statusQ.data?.data ?? [];
    const failing = components.filter((c) => !c.ok);

    return (
        <div>
            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
                <p className="max-w-2xl text-[12.5px] leading-relaxed text-muted-foreground">
                    Live health probes against the platform&apos;s backing services and its own
                    mail transport, run by the backend on each refresh.
                </p>
                <div className="flex items-center gap-2">
                    {statusQ.data && (
                        <span className="text-xs text-subtle-foreground tabular-nums">
                            Last checked {new Date(statusQ.data.checked_at).toLocaleTimeString()}
                        </span>
                    )}
                    <Button
                        size="sm"
                        variant="outline"
                        onClick={() => statusQ.refetch()}
                        disabled={statusQ.isFetching}
                    >
                        <RefreshCw className={cn("size-3.5", statusQ.isFetching && "animate-spin")} />
                        {statusQ.isFetching ? "Checking..." : "Run checks"}
                    </Button>
                </div>
            </div>

            {statusQ.isLoading && (
                <div className="space-y-3">
                    <Skeleton className="h-11 w-full" />
                    <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
                        <Skeleton className="h-64 w-full" />
                        <Skeleton className="h-64 w-full" />
                    </div>
                </div>
            )}

            {statusQ.isError && (
                <ErrorState
                    error={statusQ.error}
                    title="Could not run health checks"
                    onRetry={() => statusQ.refetch()}
                />
            )}

            {statusQ.data && (
                <>
                    {failing.length === 0 ? (
                        <Callout tone="success" icon={CheckCircle2} title="All systems operational" className="mb-5" />
                    ) : (
                        <Callout
                            tone="danger"
                            icon={XCircle}
                            className="mb-5"
                            title={
                                failing.length === 1
                                    ? "1 component is down"
                                    : `${failing.length} components are down`
                            }
                        >
                            {failing.map((c) => titleCase(c.name)).join(", ")}
                        </Callout>
                    )}

                    <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
                        <div className="order-2 overflow-hidden surface-lit rounded-xl border border-border bg-card lg:order-1">
                            <div className="flex h-9 items-center justify-between border-b border-border px-4 text-xs font-medium text-muted-foreground">
                                <span>Service</span>
                                <span>Status</span>
                            </div>
                            {components.length === 0 ? (
                                <div className="px-4 py-8 text-center text-[13px] text-muted-foreground">
                                    The status endpoint returned no components.
                                </div>
                            ) : (
                                <ul className="divide-y divide-border/70">
                                    {components.map((c) => (
                                        <ComponentRow key={c.name} component={c} />
                                    ))}
                                </ul>
                            )}
                        </div>
                        <div className="order-1 lg:order-2">
                            <MailStatusCard />
                        </div>
                    </div>
                </>
            )}
        </div>
    );
}

function ComponentRow({ component: c }: { component: SystemComponentStatus }) {
    return (
        <li className="px-4 py-3 transition-colors hover:bg-accent/50">
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                    <StatusDot tone={c.ok ? "success" : "danger"} className="font-medium text-foreground">
                        <span className="truncate">{titleCase(c.name)}</span>
                    </StatusDot>
                    <p className="mt-0.5 pl-3.5 text-xs leading-relaxed text-muted-foreground">
                        {COMPONENT_BLURBS[c.name] ?? GENERIC_BLURB}
                    </p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                    <span className="text-xs text-muted-foreground tabular-nums" title="Latency">
                        {c.latency_ms} ms
                    </span>
                    <StatusBadge tone={c.ok ? "success" : "danger"} className="w-[5.75rem] justify-center">
                        {c.ok ? "Operational" : "Down"}
                    </StatusBadge>
                </div>
            </div>
            {c.error && (
                <div className="mt-2 ml-3.5 break-words rounded-md border border-red-500/20 bg-red-500/[0.06] px-2.5 py-1.5 font-mono text-[11.5px] text-red-700 dark:text-red-400">
                    {c.error}
                </div>
            )}
        </li>
    );
}
