// Live-activity primitives for the assistant and the inline AI surfaces, after
// the agent patterns in Beautiful UI (beautifului.dev, MIT): a shimmering
// status line with an elapsed timer, a collapsible trace of tool steps, and the
// approval card that pauses a run.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { CheckIcon, ChevronDownIcon, CopyIcon, ExternalLinkIcon, KeyRoundIcon, ShieldCheckIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentPending, AgentToolStep } from "@/stores/slices/agentSlice";
import AgentMark from "./AgentMark";
import { toolAction, toolLabel } from "./toolLabels";

const EASE_OUT = [0.23, 1, 0.32, 1] as const;

function formatElapsed(ms: number): string {
    const s = ms / 1000;
    if (s < 60) return `${s.toFixed(1)}s`;
    return `${Math.floor(s / 60)}m ${Math.floor(s % 60)}s`;
}

export function Elapsed({ since, className }: { since: number; className?: string }) {
    const [now, setNow] = React.useState(() => Date.now());
    React.useEffect(() => {
        const t = window.setInterval(() => setNow(Date.now()), 100);
        return () => window.clearInterval(t);
    }, []);
    return (
        <span className={cn("font-mono tabular-nums text-slate-400", className)}>
            {formatElapsed(Math.max(0, now - since))}
        </span>
    );
}

// The run's status line: what the agent is doing right now, how long it has
// been at it, and how far into its step budget it is.
export function WorkingStatus({
    label,
    since,
    step = 0,
    budget = 0,
}: {
    label: string;
    since?: number | null;
    step?: number;
    budget?: number;
}) {
    return (
        <motion.div
            role="status"
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.2, ease: EASE_OUT }}
            className="flex items-center gap-2.5 text-[12.5px]"
        >
            <AgentMark variant="bare" size={16} state="thinking" />
            <AnimatePresence mode="wait" initial={false}>
                <motion.span
                    key={label}
                    initial={{ opacity: 0, y: 3 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -3 }}
                    transition={{ duration: 0.15 }}
                    className="ai-shimmer-text font-medium"
                >
                    {label}
                </motion.span>
            </AnimatePresence>
            {since != null && <Elapsed since={since} className="text-[11.5px]" />}
            {step > 0 && budget > 0 && (
                <span className="font-mono text-[11px] tabular-nums text-slate-300">
                    step {step}/{budget}
                </span>
            )}
        </motion.div>
    );
}

function StepRow({ step }: { step: AgentToolStep }) {
    return (
        <>
            <StepLine step={step} />
            {step.secrets && Object.keys(step.secrets).length > 0 && <SecretBox secrets={step.secrets} />}
        </>
    );
}

// A secret a step returned: shown once here, never to Remie and never stored.
function SecretBox({ secrets }: { secrets: Record<string, string> }) {
    const [copied, setCopied] = React.useState<string | null>(null);
    return (
        <div className="my-1 ml-5 rounded-md border border-amber-200 bg-amber-50/60 px-2.5 py-2">
            <div className="flex items-center gap-1.5 text-[11.5px] font-medium text-amber-800">
                <KeyRoundIcon className="size-3" />
                Shown once. Copy it now; Remie cannot see it.
            </div>
            {Object.entries(secrets).map(([k, v]) => (
                <div key={k} className="mt-1.5 flex items-center gap-2">
                    <span className="shrink-0 text-[10px] uppercase tracking-[0.14em] text-amber-700">{k}</span>
                    <code className="min-w-0 flex-1 truncate rounded bg-white px-1.5 py-0.5 font-mono text-[11.5px] text-slate-800 ring-1 ring-amber-200">
                        {v}
                    </code>
                    <button
                        type="button"
                        onClick={() => {
                            void navigator.clipboard?.writeText(v);
                            setCopied(k);
                        }}
                        className="h-6 shrink-0 rounded-md px-1.5 text-[11.5px] text-amber-800 hover:bg-amber-100 inline-flex items-center gap-1 transition-colors"
                    >
                        {copied === k ? <CheckIcon className="size-3" /> : <CopyIcon className="size-3" />}
                        {copied === k ? "Copied" : "Copy"}
                    </button>
                </div>
            ))}
        </div>
    );
}

function StepLine({ step }: { step: AgentToolStep }) {
    const detail = step.result || step.argsSummary;
    return (
        <motion.div
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: EASE_OUT }}
            className="flex min-h-7 items-center gap-2 py-0.5 text-[12px]"
        >
            {step.done ? (
                <motion.span
                    initial={{ scale: 0.5, opacity: 0 }}
                    animate={{ scale: 1, opacity: 1 }}
                    transition={{ type: "spring", stiffness: 600, damping: 24 }}
                    className="inline-flex size-3.5 shrink-0 items-center justify-center rounded-full bg-emerald-50 text-emerald-600"
                >
                    <CheckIcon className="size-2.5" strokeWidth={3} />
                </motion.span>
            ) : (
                <span className="size-3.5 shrink-0 animate-spin rounded-full border-[1.5px] border-slate-200 border-t-sky-500" />
            )}
            <span
                className={cn(
                    "shrink-0 font-medium",
                    step.done ? "text-slate-700" : "ai-shimmer-text",
                )}
            >
                {toolLabel(step.tool, step.done)}
            </span>
            {detail && (
                <span className="min-w-0 truncate text-[11.5px] text-slate-400" title={detail}>
                    {detail}
                </span>
            )}
        </motion.div>
    );
}

// A run of consecutive tool calls. One call renders as a single row; several
// fold under a header that stays open while the agent works and collapses to
// a one-line summary once it moves on, unless the user has toggled it.
export function ActivityTrace({
    steps,
    live,
    onOpen,
}: {
    steps: AgentToolStep[];
    live: boolean;
    onOpen: (url: string) => void;
}) {
    const [userOpen, setUserOpen] = React.useState<boolean | null>(null);
    const running = steps.some((s) => !s.done);
    const multi = steps.length > 1;
    const open = !multi || (userOpen ?? (running || live));
    const current = steps.find((s) => !s.done);
    const doneCount = steps.filter((s) => s.done).length;
    const summary = Array.from(new Set(steps.map((s) => toolLabel(s.tool)))).join(", ");
    const drafts = steps.filter(
        (s) =>
            s.done &&
            s.openURL &&
            (s.entityType === "campaign" || s.entityType === "automation"),
    );

    return (
        <div>
            {multi && (
                <button
                    type="button"
                    aria-expanded={open}
                    onClick={() => setUserOpen(!open)}
                    className="group -mx-1.5 flex h-7 max-w-[calc(100%+12px)] items-center gap-2 rounded-md px-1.5 text-[12px] transition-colors hover:bg-slate-100"
                >
                    <AgentMark
                        variant="bare"
                        size={14}
                        state={running ? "thinking" : "idle"}
                        tone={running ? "color" : "muted"}
                    />
                    {current ? (
                        <span className="ai-shimmer-text shrink-0 font-medium">
                            {toolLabel(current.tool, false)}
                        </span>
                    ) : (
                        <motion.span
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            transition={{ duration: 0.35 }}
                            className="shrink-0 font-medium text-slate-600"
                        >
                            Ran {steps.length} steps
                        </motion.span>
                    )}
                    {running ? (
                        <span className="shrink-0 font-mono text-[11px] tabular-nums text-slate-400">
                            {doneCount}/{steps.length}
                        </span>
                    ) : (
                        !open && (
                            <span className="min-w-0 truncate text-[11.5px] text-slate-400">
                                {summary}
                            </span>
                        )
                    )}
                    <ChevronDownIcon
                        className={cn(
                            "size-3.5 shrink-0 text-slate-400 transition-transform duration-300",
                            open && "rotate-180",
                        )}
                    />
                </button>
            )}
            <div
                className="grid transition-[grid-template-rows,opacity] duration-300 ease-[cubic-bezier(0.23,1,0.32,1)]"
                style={{ gridTemplateRows: open ? "1fr" : "0fr", opacity: open ? 1 : 0 }}
                inert={!open}
            >
                <div className="overflow-hidden">
                    <div className={cn("relative", multi && "ml-[6px] pl-3.5")}>
                        {multi && (
                            <span
                                aria-hidden
                                className="absolute bottom-2 left-0 top-1 w-px bg-slate-200"
                            />
                        )}
                        {steps.map((s) => (
                            <StepRow key={s.id} step={s} />
                        ))}
                    </div>
                </div>
            </div>
            {drafts.map((s) => (
                <motion.button
                    key={s.id}
                    initial={{ opacity: 0, y: 4 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.25, ease: EASE_OUT }}
                    onClick={() => onOpen(s.openURL!)}
                    className="mt-1.5 h-7 px-2.5 rounded-md bg-white border border-slate-200 hover:border-sky-400 hover:text-sky-700 text-[12px] text-slate-700 inline-flex items-center gap-1.5 transition-colors"
                >
                    <ExternalLinkIcon className="w-3 h-3" />
                    Open {s.entityType === "campaign" ? "campaign" : "automation"} draft
                </motion.button>
            ))}
        </div>
    );
}

// A labelled block of verbatim text, scrolled when long. HTML is shown as source, never rendered.
function Verbatim({ label, text }: { label: string; text: string }) {
    return (
        <div className="mt-2">
            <div className="text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">{label}</div>
            <pre className="mt-1 max-h-48 overflow-auto whitespace-pre-wrap break-words rounded-md border border-slate-200 bg-slate-50 px-2 py-1.5 font-mono text-[11.5px] leading-relaxed text-slate-700">
                {text}
            </pre>
        </div>
    );
}

// The pause a write or send asks for. Nothing runs until one of the buttons
// is pressed. "Always allow" appears only when the server offers it: to a
// settings manager, for a write that does not always ask.
export function ApprovalCard({
    pending,
    onDecide,
}: {
    pending: AgentPending;
    onDecide: (d: "approve" | "deny" | "always_allow") => void;
}) {
    const isSend = pending.risk === "send";
    const preview = pending.preview;
    const [argsOpen, setArgsOpen] = React.useState(!preview);
    return (
        <motion.div
            initial={{ opacity: 0, y: 6, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            transition={{ type: "spring", stiffness: 520, damping: 34 }}
            className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-[0_8px_24px_-12px_rgba(15,23,42,0.25)]"
        >
            <div className="flex h-8 items-center gap-2 border-b border-amber-100 bg-amber-50/70 px-3">
                <AgentMark variant="bare" size={14} state="attention" />
                <span className="text-[11.5px] font-medium text-amber-800">
                    {isSend ? "Ready to send" : "Waiting for your approval"}
                </span>
                <span className="ml-auto rounded bg-white/70 px-1.5 py-px text-[10px] font-semibold uppercase tracking-[0.14em] text-amber-700 ring-1 ring-amber-200">
                    {isSend ? "Sends email" : "Changes data"}
                </span>
            </div>
            <div className="px-3 py-2.5">
                <div className="flex items-center gap-1.5 text-[12.5px] font-medium text-slate-900">
                    <ShieldCheckIcon className="size-3.5 shrink-0 text-slate-400" />
                    {toolAction(pending.tool)}
                </div>
                {preview && (
                    <>
                        <dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[12px]">
                            <dt className="text-slate-500">From</dt>
                            <dd className="min-w-0 break-words font-medium text-slate-800">{preview.from}</dd>
                            <dt className="text-slate-500">To</dt>
                            <dd className="min-w-0 break-words font-medium text-slate-800">{preview.to.join(", ")}</dd>
                            <dt className="text-slate-500">Subject</dt>
                            <dd className="min-w-0 break-words text-slate-800">{preview.subject || "(none)"}</dd>
                        </dl>
                        <Verbatim label="Message" text={preview.body} />
                        {preview.body_html && <Verbatim label="HTML body" text={preview.body_html} />}
                    </>
                )}
                {pending.arguments ? (
                    <div className="mt-2">
                        <button
                            type="button"
                            aria-expanded={argsOpen}
                            onClick={() => setArgsOpen(!argsOpen)}
                            className="-mx-1 inline-flex h-6 items-center gap-1 rounded px-1 text-[11.5px] font-medium text-slate-600 hover:bg-slate-100"
                        >
                            All arguments
                            <ChevronDownIcon
                                className={cn("size-3 transition-transform duration-200", argsOpen && "rotate-180")}
                            />
                        </button>
                        {argsOpen && (
                            <pre className="mt-1 max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-md border border-slate-200 bg-slate-50 px-2 py-1.5 font-mono text-[11.5px] leading-relaxed text-slate-600">
                                {pending.arguments}
                            </pre>
                        )}
                        {pending.argumentsTruncated && (
                            <p className="mt-1 text-[11px] text-amber-700">
                                These arguments are too long to show in full. Skip if you cannot verify them.
                            </p>
                        )}
                    </div>
                ) : (
                    pending.argsSummary && (
                        <div className="mt-1.5 whitespace-pre-wrap break-words rounded-md border border-slate-200 bg-slate-50 px-2 py-1.5 font-mono text-[11.5px] leading-relaxed text-slate-600">
                            {pending.argsSummary}
                        </div>
                    )
                )}
            </div>
            <div className="flex flex-wrap items-center gap-2 px-3 pb-3">
                <button
                    onClick={() => onDecide("approve")}
                    className="h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors"
                >
                    <CheckIcon className="w-3 h-3" />
                    {isSend ? "Send" : "Approve"}
                </button>
                <button
                    onClick={() => onDecide("deny")}
                    className="h-7 px-3 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] text-slate-700 transition-colors"
                >
                    Skip
                </button>
                {!isSend && pending.alwaysAllowOffered && (
                    <button
                        title="Run this kind of action without asking, for everyone in the workspace"
                        onClick={() => onDecide("always_allow")}
                        className="ml-auto h-7 px-2.5 rounded-md text-[12px] text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-colors"
                    >
                        Always allow
                    </button>
                )}
            </div>
        </motion.div>
    );
}
