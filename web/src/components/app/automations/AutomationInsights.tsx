// The automation builder's test and history panels: a dry run against an
// editable sample event, and the record of past runs.

"use client";

import React from "react";
import { motion } from "framer-motion";
import { CheckCircle2Icon, CheckIcon, Loader2Icon, PlayIcon, XCircleIcon, XIcon } from "lucide-react";
import { useAutomationRuns } from "@/lib/api/hooks/app/automations/useAutomationRuns";
import type { AutomationNodeResult, AutomationRun, DryRunResponse } from "@/lib/api/models/app/automations/Automation";
import { actionLabel, sampleEventData } from "@/lib/api/models/app/automations/meta";
import { cn } from "@/lib/utils";
// ── Insights panel: dry-run trace + run history ─────────────────────────────
function nodeStatusIcon(status: string) {
    if (status === "error") return <XCircleIcon className="w-3.5 h-3.5 text-rose-500" />;
    if (status === "branch_true") return <CheckCircle2Icon className="w-3.5 h-3.5 text-emerald-500" />;
    if (status === "branch_false") return <XCircleIcon className="w-3.5 h-3.5 text-slate-400" />;
    if (status === "skipped") return <span className="inline-block w-3.5 h-3.5 rounded-full border border-slate-300" aria-hidden />;
    return <CheckCircle2Icon className="w-3.5 h-3.5 text-emerald-500" />;
}

function NodeResultRow({ r }: { r: AutomationNodeResult }) {
    return (
        <div className="rounded-md border border-slate-200 px-2.5 py-1.5">
            <div className="flex items-center gap-1.5">
                {nodeStatusIcon(r.status)}
                <span className={cn("text-[11.5px] font-medium", r.status === "skipped" ? "text-slate-400" : "text-slate-700")}>
                    {r.type === "condition" ? "IF" : r.type === "action" ? actionLabel(r.action ?? "") : r.type}
                </span>
                {r.type === "condition" && (
                    <span className="ml-auto text-[10.5px] font-medium text-slate-400">
                        {r.status === "branch_true" ? "→ yes" : "→ no"}
                    </span>
                )}
                {r.type === "action" && r.status === "skipped" && (
                    <span className="ml-auto text-[10.5px] font-medium text-slate-400">skipped</span>
                )}
            </div>
            {r.label && r.type === "condition" && <div className="mt-0.5 text-[11px] text-slate-400">{r.label}</div>}
            {r.error && <div className="mt-0.5 text-[11px] text-rose-600">{r.error}</div>}
            {r.preview && Object.keys(r.preview).length > 0 && (
                <div className="mt-1 space-y-0.5">
                    {Object.entries(r.preview).map(([k, v]) => (
                        <div key={k} className="text-[10.5px] text-slate-500">
                            <span className="text-slate-400">{k}:</span> <span className="font-mono">{String(v)}</span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

export function InsightsPanel({
    mode,
    automationId,
    trigger,
    steps,
    testResult,
    testing,
    onRun,
    onClose,
}: {
    mode: "test" | "history";
    automationId: string;
    trigger: string;
    steps: { id: string; label: string }[];
    testResult: DryRunResponse | null;
    testing: boolean;
    onRun: (data: Record<string, unknown>, skipNodeIds: string[]) => void;
    onClose: () => void;
}) {
    const runs = useAutomationRuns(automationId, mode === "history");

    // Editable sample event the dry-run evaluates against, seeded per trigger and
    // re-seeded when the trigger changes (its payload shape changes with it).
    const [sample, setSample] = React.useState<string>(() => JSON.stringify(sampleEventData(trigger), null, 2));
    const [sampleErr, setSampleErr] = React.useState<string | null>(null);
    // Action steps the user toggled OFF for this test (skipped in the dry-run).
    const [disabled, setDisabled] = React.useState<Set<string>>(new Set());
    React.useEffect(() => {
        setSample(JSON.stringify(sampleEventData(trigger), null, 2));
        setSampleErr(null);
    }, [trigger]);
    const resetSample = () => {
        setSample(JSON.stringify(sampleEventData(trigger), null, 2));
        setSampleErr(null);
    };
    const toggleStep = (id: string) =>
        setDisabled((prev) => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            return next;
        });
    const runWithSample = () => {
        let parsed: unknown;
        try {
            parsed = JSON.parse(sample);
        } catch (e) {
            setSampleErr((e as Error).message || "Invalid JSON");
            return;
        }
        if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
            setSampleErr("Sample data must be a JSON object.");
            return;
        }
        setSampleErr(null);
        onRun(parsed as Record<string, unknown>, [...disabled]);
    };

    return (
        <div className="flex h-full min-h-0 flex-col">
            <div className="h-11 px-3 flex items-center border-b border-slate-200 shrink-0">
                <span className="text-[12.5px] font-medium text-slate-900">{mode === "test" ? "Test run" : "Run history"}</span>
                <button
                    type="button"
                    onClick={onClose}
                    className="ml-auto h-7 w-7 rounded-md inline-flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-100"
                    aria-label="Close"
                >
                    <XIcon className="w-4 h-4" />
                </button>
            </div>

            <motion.div
                key={mode}
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.15 }}
                className="flex-1 overflow-auto p-3 space-y-2"
            >
                {mode === "test" ? (
                    <div className="space-y-3">
                        <p className="text-[11px] text-slate-400 leading-relaxed">
                            Dry run: no messages are sent and no records change. Edit the sample event your automation receives, then run to see the path it takes and what each action would send.
                        </p>
                        <div>
                            <div className="mb-1 flex items-center justify-between">
                                <span className="text-[11px] font-medium text-slate-600">Sample event data</span>
                                <button
                                    type="button"
                                    onClick={resetSample}
                                    className="text-[11px] text-sky-600 hover:text-sky-700"
                                >
                                    Reset to sample
                                </button>
                            </div>
                            <textarea
                                value={sample}
                                onChange={(e) => setSample(e.target.value)}
                                spellCheck={false}
                                rows={8}
                                className={cn(
                                    "w-full rounded-md border bg-white px-2 py-1.5 text-[11.5px] font-mono text-slate-800 outline-none resize-y focus:ring-2",
                                    sampleErr
                                        ? "border-rose-300 focus:border-rose-400 focus:ring-rose-100"
                                        : "border-slate-200 focus:border-sky-400 focus:ring-sky-100",
                                )}
                            />
                            {sampleErr ? (
                                <p className="mt-1 text-[10.5px] text-rose-600">{sampleErr}</p>
                            ) : (
                                <p className="mt-1 text-[10.5px] text-slate-400">
                                    Conditions branch on these fields, so editing them changes which actions run.
                                </p>
                            )}
                        </div>
                        {steps.length > 0 && (
                            <div>
                                <div className="mb-1 text-[11px] font-medium text-slate-600">
                                    Steps to run <span className="text-slate-400">({steps.length - disabled.size}/{steps.length})</span>
                                </div>
                                <div className="space-y-1">
                                    {steps.map((s) => {
                                        const on = !disabled.has(s.id);
                                        return (
                                            <button
                                                key={s.id}
                                                type="button"
                                                onClick={() => toggleStep(s.id)}
                                                className="w-full flex items-center gap-2 rounded-md border border-slate-200 px-2 py-1.5 text-left hover:border-slate-300"
                                            >
                                                <span
                                                    className={cn(
                                                        "flex h-4 w-4 shrink-0 items-center justify-center rounded",
                                                        on ? "bg-sky-600 text-white" : "border border-slate-300",
                                                    )}
                                                >
                                                    {on && <CheckIcon className="w-3 h-3" />}
                                                </span>
                                                <span className={cn("text-[12px]", on ? "text-slate-700" : "text-slate-400 line-through")}>
                                                    {s.label}
                                                </span>
                                            </button>
                                        );
                                    })}
                                </div>
                                <p className="mt-1 text-[10.5px] text-slate-400">
                                    Turn a step off to skip it in this test. Conditions still decide which steps are reached.
                                </p>
                            </div>
                        )}
                        <button
                            type="button"
                            onClick={runWithSample}
                            disabled={testing}
                            className="h-8 w-full rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-60"
                        >
                            {testing ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <PlayIcon className="w-3.5 h-3.5" />}
                            {testing ? "Running…" : "Run test"}
                        </button>
                        <div className="border-t border-slate-200 pt-3 space-y-2">
                            {testing ? (
                                <div className="flex items-center gap-2 text-[12px] text-slate-400">
                                    <Loader2Icon className="w-4 h-4 animate-spin" /> Running…
                                </div>
                            ) : testResult ? (
                                testResult.trace.length === 0 ? (
                                    <p className="text-[12px] text-slate-500">No actions ran for this sample (check your conditions).</p>
                                ) : (
                                    testResult.trace.map((r, i) => <NodeResultRow key={`${r.node_id}-${i}`} r={r} />)
                                )
                            ) : (
                                <p className="text-[12px] text-slate-500">Edit the sample event, then Run test.</p>
                            )}
                        </div>
                    </div>
                ) : runs.isLoading ? (
                    <div className="flex items-center gap-2 text-[12px] text-slate-400">
                        <Loader2Icon className="w-4 h-4 animate-spin" /> Loading…
                    </div>
                ) : (runs.data?.runs.length ?? 0) === 0 ? (
                    <p className="text-[12px] text-slate-500">No runs yet. This automation hasn't fired.</p>
                ) : (
                    runs.data!.runs.map((run: AutomationRun) => (
                        <div key={run.id} className="rounded-md border border-slate-200 p-2 space-y-1">
                            <div className="flex items-center gap-1.5 min-w-0">
                                {run.status === "error" ? (
                                    <XCircleIcon className="w-3.5 h-3.5 shrink-0 text-rose-500" />
                                ) : (
                                    <CheckCircle2Icon className="w-3.5 h-3.5 shrink-0 text-emerald-500" />
                                )}
                                <span className="min-w-0 truncate text-[11.5px] font-medium text-slate-700 capitalize">{run.status}</span>
                                <span className="ml-auto shrink-0 whitespace-nowrap tabular-nums text-[10.5px] text-slate-400">{new Date(run.started_at).toLocaleString()}</span>
                            </div>
                            {run.node_results?.filter((r) => r.type === "action").map((r, i) => (
                                <div key={`${run.id}-${i}`} className="pl-1">
                                    <div className="flex items-center gap-1.5">
                                        {r.status === "error" ? (
                                            <XCircleIcon className="w-3 h-3 text-rose-400" />
                                        ) : (
                                            <CheckCircle2Icon className="w-3 h-3 text-emerald-400" />
                                        )}
                                        <span className="text-[11px] text-slate-500">{actionLabel(r.action ?? "")}</span>
                                        {r.error && <span className="text-[10.5px] text-rose-500 truncate">· {r.error}</span>}
                                    </div>
                                    {r.preview && Object.keys(r.preview).length > 0 && (
                                        <div className="mt-0.5 pl-4 space-y-0.5">
                                            {Object.entries(r.preview).map(([k, v]) => (
                                                <div key={k} className="text-[10px] text-slate-400 truncate">
                                                    <span className="text-slate-300">{k}:</span>{" "}
                                                    <span className="font-mono">{String(v)}</span>
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                </div>
                            ))}
                        </div>
                    ))
                )}
            </motion.div>
        </div>
    );
}
