// Overview: is the connection healthy, how much API it uses, what is moving.

import {
    AlertTriangleIcon,
    CheckCircle2Icon,
    ClockIcon,
    Loader2Icon,
    PowerIcon,
    ShieldCheckIcon,
    XCircleIcon,
} from "lucide-react";
import toast from "react-hot-toast/headless";

import { useSalesforcePermissionCheck } from "@/lib/api/hooks/app/integrations/useSalesforce";
import type { SalesforceOverview } from "@/lib/api/models/app/integrations/Salesforce";
import { cn } from "@/lib/utils";

import StatusPill, { HealthDot } from "../../_components/StatusPill";
import { CardRow, Pill, SettingsCard, primaryBtn, secondaryBtn } from "./shared";
import { absolute, ago, errMsg } from "./util";

export default function OverviewTab({
    connectionId,
    overview,
    loading,
    onEnable,
    enabling,
    onOpenActivity,
}: {
    connectionId: string;
    overview?: SalesforceOverview;
    loading: boolean;
    onEnable: () => void;
    enabling: boolean;
    onOpenActivity: (status?: string) => void;
}) {
    const check = useSalesforcePermissionCheck(connectionId);

    if (loading || !overview) {
        return (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                {[0, 1, 2, 3].map((i) => (
                    <div key={i} className="h-20 rounded-md bg-slate-100 animate-pulse" />
                ))}
            </div>
        );
    }

    const { api, counts, org } = overview;
    const checks = overview.checks ?? check.data?.checks;

    function runCheck() {
        check.mutate(undefined, {
            onError: (err) => toast.error(errMsg(err, "Could not run the permission check")),
        });
    }

    return (
        <div className="space-y-6 max-w-5xl">
            {!overview.settings_enabled && (
                <div className="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 flex flex-wrap items-center gap-3">
                    <PowerIcon className="w-4 h-4 text-amber-600 shrink-0" />
                    <div className="min-w-0 flex-1">
                        <p className="text-[12.5px] font-medium text-amber-900">Sync is off for this connection</p>
                        <p className="text-[11.5px] text-amber-800/90 mt-0.5">
                            Nothing is logged to Salesforce and nothing is read back until you turn it on. Imports still run.
                        </p>
                    </div>
                    <button type="button" onClick={onEnable} disabled={enabling} className={primaryBtn}>
                        {enabling ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <PowerIcon className="w-3.5 h-3.5" />}
                        Enable sync
                    </button>
                </div>
            )}

            <div className="grid gap-3 lg:grid-cols-2">
                <SettingsCard title="Connection health">
                    <CardRow className="space-y-2.5">
                        <div className="flex items-center justify-between gap-2">
                            <StatusPill status={overview.status} />
                            <span className="inline-flex items-center gap-1.5 text-[11.5px] text-slate-600">
                                <HealthDot health={overview.health} />
                                {overview.health}
                            </span>
                        </div>
                        {overview.health_detail && (
                            <p className="text-[11.5px] text-slate-500 leading-relaxed">{overview.health_detail}</p>
                        )}
                        <InfoRow label="Organization" value={org.account || "Unknown"} />
                        <InfoRow
                            label="Environment"
                            value={
                                org.environment === "sandbox" ? <Pill tone="amber">Sandbox</Pill> : "Production"
                            }
                        />
                        <InfoRow label="Instance" value={org.instance_url.replace(/^https?:\/\//, "")} mono />
                        <InfoRow label="Login host" value={org.login_host} mono />
                        {org.id && <InfoRow label="Org id" value={org.id} mono />}
                    </CardRow>
                </SettingsCard>

                <SettingsCard title="API usage">
                    <CardRow className="space-y-4">
                        <Meter
                            label="Salesforce org, last 24 hours"
                            used={api.used}
                            max={api.max}
                            hint="Every integration in your org shares this allocation."
                        />
                        <Meter
                            label="Warmbly today"
                            used={api.calls_today}
                            max={api.budget}
                            hint={
                                api.budget > 0
                                    ? "Warmbly stops non-urgent calls at this budget and resumes tomorrow."
                                    : "Automatic budget: a fifth of the org's daily allocation."
                            }
                        />
                    </CardRow>
                </SettingsCard>
            </div>

            <section className="space-y-2">
                <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Sync</div>
                <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2">
                    <Stat label="Linked records" value={counts.linked_records} />
                    <Stat label="Synced, 24h" value={counts.synced_24h} tone="emerald" />
                    <Stat label="Pending" value={counts.pending} onClick={() => onOpenActivity("pending")} />
                    <Stat
                        label="Failed"
                        value={counts.failed}
                        tone={counts.failed > 0 ? "rose" : undefined}
                        onClick={() => onOpenActivity("failed")}
                        cta={counts.failed > 0 ? "View in activity log" : undefined}
                    />
                    <Stat label="Skipped, 24h" value={counts.skipped_24h} onClick={() => onOpenActivity("skipped")} />
                </div>
                <div className="rounded-md border border-slate-200 bg-white px-4 py-2.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11.5px] text-slate-600">
                    <span className="inline-flex items-center gap-1.5">
                        <ClockIcon className="w-3 h-3 text-slate-400" />
                        Last read from Salesforce{" "}
                        <span className="text-slate-900" title={absolute(overview.last_pull_at)}>
                            {ago(overview.last_pull_at)}
                        </span>
                    </span>
                    {overview.last_pull_error && (
                        <span className="inline-flex items-start gap-1.5 text-rose-700 min-w-0">
                            <AlertTriangleIcon className="w-3 h-3 mt-0.5 shrink-0" />
                            <span className="break-words">{overview.last_pull_error}</span>
                        </span>
                    )}
                </div>
            </section>

            <SettingsCard
                title="Permission check"
                description="Confirms the connected user can read and write everything the sync needs: Leads, Contacts, Tasks, Campaigns and the opt-out field."
                action={
                    <button type="button" onClick={runCheck} disabled={check.isPending} className={secondaryBtn}>
                        {check.isPending ? (
                            <Loader2Icon className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                            <ShieldCheckIcon className="w-3.5 h-3.5" />
                        )}
                        Run permission check
                    </button>
                }
            >
                {!checks || checks.length === 0 ? (
                    <CardRow>
                        <p className="text-[11.5px] text-slate-400">
                            {check.isPending ? "Checking…" : "Not run yet. It takes a few API calls."}
                        </p>
                    </CardRow>
                ) : (
                    checks.map((c) => (
                        <CardRow key={c.key} className="py-2 flex items-start gap-2">
                            {c.ok ? (
                                <CheckCircle2Icon className="w-3.5 h-3.5 text-emerald-500 mt-0.5 shrink-0" />
                            ) : (
                                <XCircleIcon className="w-3.5 h-3.5 text-rose-500 mt-0.5 shrink-0" />
                            )}
                            <div className="min-w-0 flex-1">
                                <div className="text-[12px] text-slate-800">{c.label}</div>
                                {c.detail && (
                                    <div className={cn("text-[11px] mt-0.5 break-words", c.ok ? "text-slate-500" : "text-rose-700")}>
                                        {c.detail}
                                    </div>
                                )}
                            </div>
                        </CardRow>
                    ))
                )}
            </SettingsCard>
        </div>
    );
}

function InfoRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
    return (
        <div className="flex items-center justify-between gap-3">
            <span className="text-[10.5px] uppercase tracking-[0.1em] text-slate-400 shrink-0">{label}</span>
            <span className={cn("text-[12px] text-slate-700 truncate min-w-0 text-right", mono && "font-mono text-[11px]")}>
                {value}
            </span>
        </div>
    );
}

function Meter({ label, used, max, hint }: { label: string; used: number; max: number; hint?: string }) {
    const pct = max > 0 ? Math.min(100, Math.round((used / max) * 100)) : 0;
    const tone = pct >= 90 ? "bg-rose-500" : pct >= 70 ? "bg-amber-500" : "bg-sky-600";
    return (
        <div>
            <div className="flex items-baseline justify-between gap-2">
                <span className="text-[12px] text-slate-700">{label}</span>
                <span className="text-[11.5px] tabular-nums text-slate-500">
                    <span className="text-slate-900 font-medium">{used.toLocaleString()}</span>
                    {max > 0 ? ` / ${max.toLocaleString()}` : ""}
                    {max > 0 && <span className="ml-1.5 text-slate-400">{pct}%</span>}
                </span>
            </div>
            <div className="mt-1.5 h-1.5 rounded-full bg-slate-100 overflow-hidden">
                <div className={cn("h-full rounded-full transition-all", tone)} style={{ width: `${pct}%` }} />
            </div>
            {hint && <p className="text-[10.5px] text-slate-400 mt-1">{hint}</p>}
        </div>
    );
}

function Stat({
    label,
    value,
    tone,
    onClick,
    cta,
}: {
    label: string;
    value: number;
    tone?: "emerald" | "rose";
    onClick?: () => void;
    cta?: string;
}) {
    const valueTone = tone === "rose" ? "text-rose-700" : tone === "emerald" ? "text-emerald-700" : "text-slate-900";
    const body = (
        <>
            <div className="text-[10px] uppercase tracking-[0.12em] text-slate-500 font-medium">{label}</div>
            <div className={cn("mt-1 text-[18px] font-semibold tabular-nums leading-none", valueTone)}>
                {value.toLocaleString()}
            </div>
            {cta && <div className="mt-1.5 text-[10.5px] text-rose-700 underline underline-offset-2">{cta}</div>}
        </>
    );
    if (!onClick) return <div className="rounded-md border border-slate-200 bg-white px-3 py-2.5">{body}</div>;
    return (
        <button
            type="button"
            onClick={onClick}
            className="text-left rounded-md border border-slate-200 bg-white px-3 py-2.5 hover:border-slate-300 hover:bg-slate-50/60 transition-colors"
        >
            {body}
        </button>
    );
}
