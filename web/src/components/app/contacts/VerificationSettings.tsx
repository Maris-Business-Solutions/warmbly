// Address verification at a glance: who checks this workspace's contacts, what
// it has found so far, and the one-click path to a connected verifier. Verdicts
// change in the background, so the bar and the numbers animate as they land.

import React from "react";
import { Link } from "@tanstack/react-router";
import { motion } from "framer-motion";
import { AlertTriangleIcon, ArrowRightIcon, CoinsIcon, ShieldCheckIcon } from "lucide-react";

import { Section } from "@/app/app/settings/_components/SectionShell";
import AnimatedNumber from "@/components/ui/AnimatedNumber";
import { useContactVerification } from "@/lib/api/hooks/app/contacts/useContactVerification";
import { cn } from "@/lib/utils";
import { PROVIDER_LABELS, type IntegrationProvider } from "@/lib/api/models/app/integrations/Integration";

const SEGMENTS = [
    { key: "valid", label: "Deliverable", color: "bg-emerald-500", text: "text-emerald-700" },
    { key: "risky", label: "Risky", color: "bg-amber-400", text: "text-amber-700" },
    { key: "invalid", label: "Undeliverable", color: "bg-rose-500", text: "text-rose-700" },
    { key: "unknown", label: "Unverified", color: "bg-slate-200", text: "text-slate-500" },
] as const;

export default function VerificationSettings() {
    const { data, isLoading } = useContactVerification();
    const counts = data?.counts;
    const total = counts ? counts.valid + counts.risky + counts.invalid + counts.unknown : 0;
    const paid = !!data && data.provider !== "builtin";
    const providerName = data ? PROVIDER_LABELS[data.provider as IntegrationProvider] ?? data.provider : "";

    return (
        <Section
            eyebrow="Address verification"
            description="Every contact you add is checked before any campaign sends to it, so bad addresses never become bounces. Nothing to run: verdicts land in the background and appear on each contact."
        >
            {isLoading || !data ? (
                <div className="h-16 rounded-md bg-slate-100 animate-pulse" />
            ) : (
                <div className="space-y-4">
                    <div className="flex flex-col md:flex-row md:items-center gap-3">
                        <div className="flex items-center gap-2.5 min-w-0">
                            <motion.span
                                initial={{ scale: 0.6, opacity: 0 }}
                                animate={{ scale: 1, opacity: 1 }}
                                transition={{ type: "spring", duration: 0.5, bounce: 0.4 }}
                                className={cn(
                                    "w-8 h-8 rounded-md inline-flex items-center justify-center shrink-0",
                                    paid ? "bg-emerald-50 text-emerald-600" : "bg-sky-50 text-sky-600",
                                )}
                            >
                                <ShieldCheckIcon className="w-4 h-4" />
                            </motion.span>
                            <div className="min-w-0">
                                <p className="text-[12.5px] font-medium text-slate-900">
                                    {paid ? providerName : "Built-in check"}
                                    <span className="ml-1.5 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">
                                        {paid ? "connected service" : "included"}
                                    </span>
                                </p>
                                <p className="text-[11.5px] text-slate-500 leading-snug">
                                    {paid
                                        ? data.credits != null
                                            ? `One credit per address, from your own ${providerName} balance.`
                                            : `One check per address, from your own ${providerName} account. It publishes no balance, so view what is left in ${providerName}.`
                                        : data.builtin_ready
                                          ? "Syntax, mail server, disposable domains and a mailbox probe. Catch-all domains and Microsoft 365 stay unverified."
                                          : "Syntax, mail server and disposable-domain checks. The mailbox probe is off on this instance, so most addresses stay unverified."}
                                </p>
                            </div>
                        </div>
                        <div className="md:ml-auto shrink-0 flex items-center gap-2">
                            {paid && data.credits != null && (
                                <span className="inline-flex items-center gap-1.5 h-7 px-2.5 rounded-md bg-slate-50 border border-slate-200 text-[12px] text-slate-700">
                                    <CoinsIcon className="w-3.5 h-3.5 text-amber-500" />
                                    <AnimatedNumber value={data.credits} className="font-medium tabular-nums" /> credits
                                </span>
                            )}
                            <Link
                                to="/app/integrations"
                                className="inline-flex items-center gap-1.5 h-7 px-2.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium transition-colors"
                            >
                                {data.connection_id ? "Manage connection" : "Connect verification service"}
                                <ArrowRightIcon className="w-3.5 h-3.5" />
                            </Link>
                        </div>
                    </div>

                    {data.provider_error && (
                        <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2">
                            <AlertTriangleIcon className="w-3.5 h-3.5 text-amber-600 shrink-0 mt-0.5" />
                            <p className="text-[12px] text-amber-800 leading-snug">{data.provider_error}</p>
                        </div>
                    )}

                    <div>
                        <div className="flex h-2 w-full overflow-hidden rounded-full bg-slate-100">
                            {SEGMENTS.map((s) => {
                                const n = counts?.[s.key] ?? 0;
                                return (
                                    <motion.div
                                        key={s.key}
                                        className={s.color}
                                        initial={{ width: 0 }}
                                        animate={{ width: total ? `${(n / total) * 100}%` : "0%" }}
                                        transition={{ type: "spring", duration: 0.8, bounce: 0.1 }}
                                    />
                                );
                            })}
                        </div>
                        <div className="mt-2.5 flex flex-wrap gap-x-5 gap-y-1.5">
                            {SEGMENTS.map((s) => (
                                <div key={s.key} className="inline-flex items-center gap-1.5 text-[11.5px]">
                                    <span className={cn("w-2 h-2 rounded-full", s.color)} />
                                    <span className="text-slate-500">{s.label}</span>
                                    <AnimatedNumber value={counts?.[s.key] ?? 0} className={cn("font-medium tabular-nums", s.text)} />
                                </div>
                            ))}
                            {(counts?.pending ?? 0) > 0 && (
                                <motion.span
                                    initial={{ opacity: 0 }}
                                    animate={{ opacity: 1 }}
                                    className="inline-flex items-center gap-1.5 text-[11.5px] text-slate-400"
                                >
                                    <span className="w-2 h-2 rounded-full bg-sky-400 animate-pulse" />
                                    <AnimatedNumber value={counts?.pending ?? 0} className="tabular-nums" /> in the queue
                                </motion.span>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </Section>
    );
}
