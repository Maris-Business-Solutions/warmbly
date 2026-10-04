// What is waiting to reach the CRM, what failed and why, and when each pull
// last ran. CRM_SYNCED keeps it live.

import type { ReactNode } from "react";
import { AlertTriangleIcon, CheckCircle2Icon, Loader2Icon, RotateCcwIcon, Trash2Icon } from "lucide-react";
import toast from "react-hot-toast";

import { useConfirm } from "@/hooks/context/confirm";
import { useDiscardCrmSync, useRetryCrmSync } from "@/lib/api/hooks/app/crm/provider/useCrmSyncActions";
import useCrmSyncHealth from "@/lib/api/hooks/app/crm/provider/useCrmSyncHealth";
import { cn } from "@/lib/utils";

import { type CrmInfo } from "@/components/app/crm/crmProviders";

import { usePageCrm } from "./context";
import { Card, SubLabel } from "./editors";
import { errMessage, humanize, plural, timeAgo } from "./shared";

function counts(crm: CrmInfo) {
    const cap = (w: string) => w.charAt(0).toUpperCase() + w.slice(1);
    return [
        ["contacts", `${cap(crm.words.contacts)} linked`],
        ["deals", "Deals"],
        ["tasks", cap(crm.words.tasks)],
        ["pipelines", "Pipelines"],
        ["owners", cap(crm.words.owners)],
    ] as const;
}

// What each pull row is called, where the object name alone would not say.
const CURSOR_LABELS: Record<string, string> = {
    webhooks: "Instant updates",
    fields: "Warmbly fields",
    persons: "People",
};

export default function SyncHealthCard({ canManage }: { canManage: boolean }) {
    const crm = usePageCrm();
    const COUNTS = counts(crm);
    const health = useCrmSyncHealth();
    const retry = useRetryCrmSync();
    const discard = useDiscardCrmSync();
    const confirm = useConfirm();
    const h = health.data;
    const failures = h?.failures ?? [];
    const cursors = h?.cursors ?? [];

    function retryIds(ids: string[]) {
        retry.mutate(ids, {
            onSuccess: () => toast.success(ids.length === 1 ? "Retrying" : "Retrying every failed change"),
            onError: (err) => toast.error(errMessage(err, "Could not retry")),
        });
    }

    function discardIds(ids: string[], count: number) {
        confirm.show(
            ids.length === 0
                ? `Discard ${plural(count, "failed change")}? They will not be sent to ${crm.name}.`
                : `Discard this change? It will not be sent to ${crm.name}.`,
            async () => {
                try {
                    await discard.mutateAsync(ids);
                    toast.success("Discarded");
                } catch (err) {
                    toast.error(errMessage(err, "Could not discard"));
                }
            },
        );
    }

    return (
        <Card
            title="Sync health"
            description={`Warmbly sends its changes to ${crm.name} as they happen and pulls ${crm.name}'s changes back on its own.`}
            actions={
                canManage && failures.length > 0 ? (
                    <>
                        <button
                            type="button"
                            onClick={() => discardIds([], h?.failed ?? failures.length)}
                            className="h-7 px-2.5 rounded-md text-[12px] text-slate-500 hover:text-rose-600 hover:bg-rose-50 inline-flex items-center gap-1.5 transition-colors"
                        >
                            <Trash2Icon className="w-3 h-3" />
                            Discard all
                        </button>
                        <button
                            type="button"
                            onClick={() => retryIds([])}
                            disabled={retry.isPending}
                            className="h-7 px-2.5 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] text-slate-700 hover:text-slate-900 inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                        >
                            {retry.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <RotateCcwIcon className="w-3 h-3" />}
                            Retry all
                        </button>
                    </>
                ) : undefined
            }
        >
            {health.isLoading ? (
                <p className="text-[12px] text-slate-400 inline-flex items-center gap-1.5">
                    <Loader2Icon className="w-3 h-3 animate-spin" />
                    Checking…
                </p>
            ) : health.isError ? (
                <p className="text-[12px] text-rose-600">{errMessage(health.error, "Could not load sync health.")}</p>
            ) : (
                <>
                    <div className="grid grid-cols-3 sm:grid-cols-5 gap-px bg-slate-200/70 rounded-md border border-slate-200 overflow-hidden">
                        {COUNTS.map(([k, label]) => (
                            <div key={k} className="bg-white px-3 py-2.5">
                                <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium truncate">{label}</div>
                                <div className="text-[16px] font-semibold text-slate-900 tabular-nums mt-0.5">
                                    {(h?.counts[k] ?? 0).toLocaleString()}
                                </div>
                            </div>
                        ))}
                    </div>

                    <div className="flex flex-wrap items-center gap-1.5">
                        <Chip tone={(h?.pending ?? 0) > 0 ? "sky" : "slate"}>
                            {(h?.pending ?? 0) > 0 && <Loader2Icon className="w-3 h-3 animate-spin" />}
                            {plural(h?.pending ?? 0, "change")} waiting
                        </Chip>
                        <Chip tone={(h?.failed ?? 0) > 0 ? "rose" : "slate"}>{plural(h?.failed ?? 0, "failure")}</Chip>
                        <Chip tone="emerald">{plural(h?.done_24h ?? 0, "change")} synced in the last 24 hours</Chip>
                    </div>

                    {cursors.length > 0 && (
                        <div className="space-y-1.5">
                            <SubLabel>Pulled from {crm.name}</SubLabel>
                            <div className="rounded-md border border-slate-200 divide-y divide-slate-100">
                                {cursors.map((c) => (
                                    <div key={c.object_type} className="px-3 py-1.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[12px]">
                                        <span className="text-slate-800 w-28 shrink-0">{CURSOR_LABELS[c.object_type] ?? humanize(c.object_type)}</span>
                                        <span className="text-slate-500 tabular-nums">{c.last_run_at ? timeAgo(c.last_run_at) : "not yet"}</span>
                                        {c.last_error && (
                                            <span className="basis-full sm:basis-auto sm:flex-1 min-w-0 text-[11px] text-rose-600 truncate" title={c.last_error}>
                                                {c.last_error}
                                            </span>
                                        )}
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {failures.length > 0 ? (
                        <div className="space-y-1.5">
                            <SubLabel>Did not reach {crm.name}</SubLabel>
                            <div className="rounded-md border border-rose-200 divide-y divide-rose-100">
                                {failures.map((f) => (
                                    <div key={f.id} className="px-3 py-2 flex flex-col sm:flex-row sm:items-start gap-2">
                                        <AlertTriangleIcon className="hidden sm:block w-3.5 h-3.5 text-rose-500 mt-0.5 shrink-0" />
                                        <div className="min-w-0 flex-1">
                                            <div className="text-[12.5px] text-slate-900 truncate">{f.subject || humanize(f.kind)}</div>
                                            {f.last_error && <p className="text-[11.5px] text-rose-700 leading-relaxed break-words">{f.last_error}</p>}
                                            <p className="text-[10.5px] text-slate-400 mt-0.5">
                                                Tried {plural(f.attempts, "time")} · {timeAgo(f.updated_at)}
                                            </p>
                                        </div>
                                        {canManage && (
                                            <div className="flex items-center gap-1 shrink-0">
                                                <button
                                                    type="button"
                                                    onClick={() => retryIds([f.id])}
                                                    disabled={retry.isPending}
                                                    className="h-7 px-2 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] text-slate-700 inline-flex items-center gap-1 transition-colors disabled:opacity-60"
                                                >
                                                    <RotateCcwIcon className="w-3 h-3" />
                                                    Retry
                                                </button>
                                                <button
                                                    type="button"
                                                    onClick={() => discardIds([f.id], 1)}
                                                    aria-label="Discard this change"
                                                    title="Discard"
                                                    className="size-7 rounded-md text-slate-400 hover:text-rose-600 hover:bg-rose-50 inline-flex items-center justify-center transition-colors"
                                                >
                                                    <Trash2Icon className="w-3.5 h-3.5" />
                                                </button>
                                            </div>
                                        )}
                                    </div>
                                ))}
                            </div>
                        </div>
                    ) : (
                        <p className="text-[12px] text-emerald-700 inline-flex items-center gap-1.5">
                            <CheckCircle2Icon className="w-3.5 h-3.5" />
                            Nothing failed. Everything Warmbly changed is in {crm.name}.
                        </p>
                    )}
                </>
            )}
        </Card>
    );
}

function Chip({ tone, children }: { tone: "sky" | "rose" | "slate" | "emerald"; children: ReactNode }) {
    return (
        <span
            className={cn(
                "inline-flex items-center gap-1 h-6 px-2 rounded-md text-[11.5px] font-medium tabular-nums",
                tone === "sky" && "bg-sky-50 text-sky-700",
                tone === "rose" && "bg-rose-50 text-rose-700",
                tone === "slate" && "bg-slate-100 text-slate-600",
                tone === "emerald" && "bg-emerald-50 text-emerald-700",
            )}
        >
            {children}
        </span>
    );
}
