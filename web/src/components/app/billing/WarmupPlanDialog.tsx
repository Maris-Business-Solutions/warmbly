// The warmup plan checkout: the premium pool, with no mailbox cap, for a
// workspace that only warms. A customer arrives here from the mailboxes page,
// the billing overview, or the Upgrade button on a linked self-hosted
// instance, so they have already chosen: this confirms what they are buying,
// which workspace it applies to, and how often they are billed.
//
// The plan is not in the public plan list, so it has no card in the plans grid
// and its price comes from /pool-link/offer rather than the static catalog.
// The layout is the same anatomy as a PlanCard in the chooser (name, blurb,
// price, bullets) so it reads as one more plan, not a promotion.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import toast from "react-hot-toast/headless";
import { ArrowRightIcon, CheckIcon, Loader2Icon, MinusIcon, XIcon } from "lucide-react";
import { useAppStore } from "@/stores";
import { usePoolLinkInstances, usePoolLinkOffer, useStartPoolLinkCheckout } from "@/lib/api/hooks/app/cloudlink/useCloudLink";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import BillingIntervalToggle from "@/components/app/billing/BillingIntervalToggle";
import { DitherMeter } from "@/components/ui/dither";
import { PLAN_ACCENT_CLASSES, WARMUP_PLAN_BENEFITS, WARMUP_PLAN_PITCH, getPlan } from "@/lib/plans";

type Interval = "month" | "year";

// Free against Premium, row by row. Each cell is a rule the pool enforces.
const COMPARISON: { label: string; free: string | false; premium: string | true }[] = [
    { label: "Warmup pool", free: "Free pool", premium: "Premium pool" },
    { label: "Partner matching", free: "After Premium", premium: "First" },
    { label: "Mailboxes", free: "10", premium: "No limit" },
    { label: "Spam-prone partners removed", free: "Yes", premium: true },
    { label: "Replies and spam rescue", free: "Yes", premium: true },
];

export default function WarmupPlanDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
    const orgName = useAppStore((s) => s.currentOrganization?.name);
    const offer = usePoolLinkOffer(open);
    // The workspace's own allowance, so the right column shows why to upgrade
    // rather than a made-up chart. Hidden when it cannot be read.
    const instances = usePoolLinkInstances(open);
    const allowance = instances.data?.plan;
    const checkout = useStartPoolLinkCheckout();
    const [interval, setInterval] = React.useState<Interval>("year");

    // Yearly is the better default, but only where it exists. Applied once per
    // opening: a background refetch must not move the choice under someone who
    // has already picked the other period.
    const defaulted = React.useRef(false);
    React.useEffect(() => {
        if (!open) {
            defaulted.current = false;
            return;
        }
        if (defaulted.current || !offer.data) return;
        defaulted.current = true;
        setInterval(offer.data.yearly_available ? "year" : "month");
    }, [open, offer.data]);

    const cardRef = React.useRef<HTMLDivElement>(null);
    React.useEffect(() => {
        if (!open) return;
        const previous = document.activeElement as HTMLElement | null;
        cardRef.current?.focus();
        const onKey = (e: KeyboardEvent) => {
            if (e.key === "Escape" && !checkout.isPending) {
                e.stopPropagation();
                onClose();
            }
        };
        document.addEventListener("keydown", onKey, true);
        return () => {
            document.removeEventListener("keydown", onKey, true);
            previous?.focus?.();
        };
    }, [open, checkout.isPending, onClose]);

    const data = offer.data;
    const available = data?.available === true;
    const monthly = data?.monthly_usd ?? 0;
    const yearly = data?.yearly_usd ?? 0;
    const annual = interval === "year" && data?.yearly_available === true;
    // Only worth showing when it is actually a saving.
    const yearlySaving = monthly > 0 && yearly > 0 ? Math.max(0, monthly * 12 - yearly) : 0;
    const accent = PLAN_ACCENT_CLASSES[getPlan("warmup").accent];

    async function start() {
        if (checkout.isPending || !available) return;
        try {
            const res = await checkout.mutateAsync(interval);
            if (!res.checkout_url) throw new Error("no checkout url");
            window.location.href = res.checkout_url;
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    }

    return createPortal(
        <AnimatePresence>
            {open && (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    transition={{ duration: 0.16 }}
                    onMouseDown={(e) => {
                        if (e.target === e.currentTarget && !checkout.isPending) onClose();
                    }}
                    className="fixed inset-0 z-[160] flex items-center justify-center bg-slate-900/40 backdrop-blur-[2px] px-4"
                >
                    <motion.div
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="pool-upgrade-title"
                        data-floating
                        ref={cardRef}
                        tabIndex={-1}
                        initial={{ opacity: 0, y: 8 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: 8 }}
                        transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                        onMouseDown={(e) => e.stopPropagation()}
                        className="w-full max-w-[720px] max-h-[calc(100dvh-3rem)] overflow-y-auto rounded-lg bg-white border border-slate-200 shadow-[0_24px_48px_-12px_rgba(15,23,42,0.18)]"
                    >
                        <div className="h-12 px-4 border-b border-slate-200 flex items-center gap-2.5 sticky top-0 bg-white">
                            <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Warmup plan</span>
                            <div className="h-4 w-px bg-slate-200" />
                            <span id="pool-upgrade-title" className="text-[12.5px] text-slate-900 font-medium">
                                Premium warmup pool
                            </span>
                            <button
                                type="button"
                                onClick={onClose}
                                disabled={checkout.isPending}
                                aria-label="Close"
                                className="ml-auto size-7 rounded-md text-slate-500 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center justify-center transition-colors disabled:opacity-50"
                            >
                                <XIcon className="w-3.5 h-3.5" />
                            </button>
                        </div>

                        <div className="grid md:grid-cols-[1fr_272px]">
                            {/* Left: the plan, in the chooser's anatomy. */}
                            <div className="px-5 py-5">
                                <div className="flex items-center gap-2">
                                    <span className="text-[12px] uppercase tracking-[0.1em] font-semibold text-slate-800">Premium</span>
                                    {!offer.isLoading && available && data?.yearly_available && data?.monthly_available && (
                                        <div className="ml-auto">
                                            <BillingIntervalToggle
                                                interval={interval === "year" ? "annual" : "monthly"}
                                                onChange={(i) => setInterval(i === "annual" ? "year" : "month")}
                                            />
                                        </div>
                                    )}
                                </div>
                                <p className="mt-1.5 text-[12.5px] text-slate-500 leading-snug">{WARMUP_PLAN_PITCH}</p>

                                <div className="mt-4 flex items-baseline gap-1.5">
                                    {offer.isLoading ? (
                                        <span className="h-8 w-20 rounded bg-slate-100 animate-pulse" />
                                    ) : (
                                        <>
                                            <span className="text-[34px] font-semibold tracking-[-0.03em] text-slate-900 tabular-nums">
                                                ${annual ? Math.round(yearly / 12) : monthly}
                                            </span>
                                            <span className="text-[12px] text-slate-500">/ mo</span>
                                        </>
                                    )}
                                </div>
                                <div className="mt-1 flex items-center gap-1.5 text-[11px] text-slate-400 tabular-nums">
                                    {annual ? (
                                        <>
                                            <span>billed ${yearly} annually</span>
                                            {yearlySaving > 0 && (
                                                <span className="inline-flex items-center h-4 px-1.5 rounded-full bg-emerald-50 text-emerald-700 border border-emerald-100 text-[10px] font-semibold">
                                                    save ${yearlySaving} / yr
                                                </span>
                                            )}
                                        </>
                                    ) : (
                                        "billed monthly"
                                    )}
                                </div>

                                <ul className="mt-5 space-y-3">
                                    {WARMUP_PLAN_BENEFITS.map((b) => (
                                        <li key={b.title} className="flex items-start gap-2.5">
                                            <CheckIcon className="w-3.5 h-3.5 mt-0.5 text-emerald-600 shrink-0" aria-hidden="true" />
                                            <span className="text-[12.5px] leading-snug">
                                                <span className="font-medium text-slate-800">{b.title}.</span>{" "}
                                                <span className="text-slate-500">{b.body}</span>
                                            </span>
                                        </li>
                                    ))}
                                </ul>

                                {!offer.isLoading && !available && (
                                    <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5">
                                        <p className="text-[12px] text-amber-900">
                                            {offer.isError
                                                ? "The price could not be loaded. Try again in a moment."
                                                : "This instance has no price set for the warmup plan yet, so it cannot be bought here."}
                                        </p>
                                    </div>
                                )}

                                <p className="mt-5 text-[11.5px] text-slate-400 leading-relaxed">
                                    Applies to <span className="text-slate-600 font-medium">{orgName || "this workspace"}</span> and any instance
                                    linked to it. Billed through Stripe. Cancel anytime.
                                </p>
                            </div>

                            {/* Right: where this workspace stands, and what changes. */}
                            <aside className="border-t md:border-t-0 md:border-l border-slate-200/70 bg-slate-50/60 px-5 py-5 space-y-5">
                                {allowance && (
                                    <div>
                                        <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Your pool today</div>
                                        <div className="mt-2 flex items-baseline justify-between gap-2">
                                            <span className="text-[12.5px] text-slate-700 font-medium">Mailboxes warming</span>
                                            <span className="text-[11.5px] font-mono tabular-nums text-slate-700">
                                                {allowance.enrolled}
                                                <span className="text-slate-400">
                                                    {allowance.mailbox_limit === null ? " / no limit" : ` / ${allowance.mailbox_limit} free`}
                                                </span>
                                            </span>
                                        </div>
                                        <DitherMeter
                                            className="mt-1.5"
                                            frac={allowance.mailbox_limit ? Math.min(1, allowance.enrolled / allowance.mailbox_limit) : 0}
                                            tone={allowance.mailbox_limit && allowance.enrolled >= allowance.mailbox_limit ? "amber" : "sky"}
                                            height={5}
                                        />
                                        <p className="mt-1.5 text-[11px] text-slate-400 leading-snug">
                                            {allowance.mailbox_limit === null
                                                ? "This workspace already has no mailbox cap."
                                                : allowance.enrolled >= allowance.mailbox_limit
                                                  ? "The free allowance is used up. Premium warms every mailbox you add."
                                                  : `${allowance.mailbox_limit - allowance.enrolled} free slots left. Premium removes the cap.`}
                                        </p>
                                    </div>
                                )}

                                <div>
                                    <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">What changes</div>
                                    <table className="mt-2 w-full text-[11.5px]">
                                        <thead>
                                            <tr className="text-[10px] uppercase tracking-[0.08em] text-slate-400">
                                                <th className="font-medium text-left pb-1.5" />
                                                <th className="font-medium text-right pb-1.5 w-16">Free</th>
                                                <th className="font-medium text-right pb-1.5 w-20 text-sky-700">Premium</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-slate-200/70">
                                            {COMPARISON.map((r) => (
                                                <tr key={r.label}>
                                                    <td className="py-1.5 pr-2 text-slate-600">{r.label}</td>
                                                    <td className="py-1.5 text-right text-slate-400 tabular-nums">
                                                        {r.free === false ? <MinusIcon className="w-3 h-3 inline text-slate-300" aria-label="No" /> : r.free}
                                                    </td>
                                                    <td className="py-1.5 pl-2 text-right text-slate-900 font-medium tabular-nums">
                                                        {r.premium === true ? <CheckIcon className="w-3.5 h-3.5 inline text-emerald-600" aria-label="Yes" /> : r.premium}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </aside>
                        </div>

                        <div className="px-4 py-3 border-t border-slate-200 flex items-center justify-end gap-2">
                            <button
                                type="button"
                                onClick={onClose}
                                disabled={checkout.isPending}
                                className="h-8 px-3 rounded-md border border-slate-200 text-slate-700 text-[12.5px] font-medium hover:bg-slate-50 transition-colors disabled:opacity-50"
                            >
                                Not now
                            </button>
                            <button
                                type="button"
                                onClick={start}
                                disabled={!available || checkout.isPending}
                                className={`h-8 px-3 rounded-md text-[12.5px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-50 ${accent.button}`}
                            >
                                {checkout.isPending ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <ArrowRightIcon className="w-3.5 h-3.5" />}
                                Upgrade to Premium
                            </button>
                        </div>
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>,
        document.body,
    );
}
