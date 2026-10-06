// The "why" behind a contact's deliverability verdict: an animated
// confidence ring, who produced it, the reasons in plain words, and the
// observations they were scored from. Absence of engagement is never listed,
// because it is not evidence of anything.

import { AnimatePresence, motion } from "framer-motion";
import { Link } from "@tanstack/react-router";
import toast from "react-hot-toast/headless";
import {
    AlertTriangleIcon,
    BadgeCheckIcon,
    CircleDashedIcon,
    Loader2Icon,
    MailCheckIcon,
    MailOpenIcon,
    MailWarningIcon,
    MousePointerClickIcon,
    RefreshCcwIcon,
    ReplyIcon,
    ShieldCheckIcon,
    ShieldXIcon,
} from "lucide-react";
import type { ContactVerificationDetail, VerificationEvidenceKind } from "@/lib/api/models/app/contacts/ContactDetail";
import { useContactVerification, useRequestContactVerification } from "@/lib/api/hooks/app/contacts/useContactVerification";
import { reverifyNotice } from "@/lib/api/client/app/contacts/verification";
import { PROVIDER_LABELS, type IntegrationProvider } from "@/lib/api/models/app/integrations/Integration";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { useWriteGuard } from "@/hooks/usePermission";
import { verificationSourceLabel } from "../VerificationBadge";
import { fmtAbsolute, fmtRelative } from "./format";
import { cn } from "@/lib/utils";

const STATUS = {
    valid: { label: "Deliverable", ring: "stroke-emerald-500", text: "text-emerald-700", Icon: ShieldCheckIcon },
    risky: { label: "Risky", ring: "stroke-amber-500", text: "text-amber-700", Icon: AlertTriangleIcon },
    invalid: { label: "Undeliverable", ring: "stroke-rose-500", text: "text-rose-700", Icon: ShieldXIcon },
    unknown: { label: "Not verified", ring: "stroke-slate-300", text: "text-slate-500", Icon: CircleDashedIcon },
} as const;

const EVIDENCE: Record<VerificationEvidenceKind, { label: string; Icon: typeof MailCheckIcon; tone: string }> = {
    delivered: { label: "Delivered, no bounce", Icon: MailCheckIcon, tone: "text-emerald-600" },
    opened: { label: "Opened by a person", Icon: MailOpenIcon, tone: "text-emerald-600" },
    clicked: { label: "Clicked a link", Icon: MousePointerClickIcon, tone: "text-emerald-600" },
    replied: { label: "Replied", Icon: ReplyIcon, tone: "text-emerald-600" },
    auto_replied: { label: "Automatic reply (mailbox is live)", Icon: ReplyIcon, tone: "text-emerald-600" },
    bounced_recipient: { label: "Bounced: mailbox does not exist", Icon: MailWarningIcon, tone: "text-rose-600" },
    bounced_other: { label: "Bounced for another reason", Icon: MailWarningIcon, tone: "text-slate-500" },
};

export default function VerificationCard({
    contactId,
    detail,
    loading,
}: {
    contactId: string;
    detail?: ContactVerificationDetail | null;
    loading: boolean;
}) {
    const write = useWriteGuard("MANAGE_CONTACTS");
    const request = useRequestContactVerification();
    const pending = request.isPending || !!detail?.requested_at;
    const { data: overview } = useContactVerification(pending);

    if (loading && !detail) {
        return <div className="h-20 rounded-md border border-slate-200 bg-slate-50 animate-pulse" />;
    }
    if (!detail) return null;
    const meta = STATUS[detail.status] ?? STATUS.unknown;
    const Icon = meta.Icon;
    const r = 16;
    const c = 2 * Math.PI * r;
    const pct = Math.max(0, Math.min(100, detail.confidence));

    const source = verificationSourceLabel(detail.source, detail.provider, detail.provider_label);
    // What the check said, when real mail decided otherwise.
    const checkSaid =
        detail.check_status && detail.check_status !== detail.status && (detail.source === "provider" || detail.source === "probe")
            ? STATUS[detail.check_status]?.label.toLowerCase()
            : "";
    // Named only once the overview says who runs checks, so the card never
    // flashes the wrong verifier while it loads.
    const runner = !overview
        ? ""
        : overview.provider !== "builtin" && !overview.provider_error
          ? PROVIDER_LABELS[overview.provider as IntegrationProvider] ?? overview.provider
          : "Warmbly's built-in check";

    async function reverify() {
        try {
            const res = await request.mutateAsync({ contacts: [contactId], action: "verify" });
            const notice = reverifyNotice(res, "address", "addresses");
            if (notice.warn) toast(notice.text, { icon: "⚠️" });
            else toast.success(notice.text);
        } catch (err) {
            toast.error(buildError(err as AppError));
        }
    }

    return (
        <div className="rounded-md border border-slate-200 bg-white overflow-hidden">
            <div className="px-3 py-2.5 flex items-center gap-3">
                <div className="relative w-11 h-11 shrink-0">
                    <svg viewBox="0 0 40 40" className="w-11 h-11 -rotate-90">
                        <circle cx="20" cy="20" r={r} className="stroke-slate-100" strokeWidth="4" fill="none" />
                        <motion.circle
                            cx="20"
                            cy="20"
                            r={r}
                            className={meta.ring}
                            strokeWidth="4"
                            strokeLinecap="round"
                            fill="none"
                            strokeDasharray={c}
                            initial={{ strokeDashoffset: c }}
                            animate={{ strokeDashoffset: c - (c * pct) / 100 }}
                            transition={{ type: "spring", duration: 1, bounce: 0.15 }}
                        />
                    </svg>
                    <motion.span
                        key={detail.status}
                        initial={{ scale: 0.5, opacity: 0 }}
                        animate={{ scale: 1, opacity: 1 }}
                        transition={{ type: "spring", duration: 0.4, bounce: 0.5 }}
                        className={cn("absolute inset-0 flex items-center justify-center", meta.text)}
                    >
                        <Icon className="w-4 h-4" />
                    </motion.span>
                </div>
                <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                        <span className={cn("text-[13px] font-semibold", meta.text)}>{meta.label}</span>
                        <span className="text-[11px] text-slate-400 tabular-nums">{pct}% sure</span>
                        {detail.decisive && (
                            <span className="hidden sm:inline text-[10px] uppercase tracking-[0.12em] text-slate-400 font-medium">
                                from real mail
                            </span>
                        )}
                        <button
                            type="button"
                            onClick={(e) => write.guard(() => void reverify())(e)}
                            disabled={pending}
                            title={pending ? "A re-check is already queued" : "Check this address again now"}
                            className="ml-auto shrink-0 h-6 px-2 rounded-md border border-slate-200 bg-white text-[11px] font-medium text-slate-700 hover:bg-slate-50 hover:border-slate-300 inline-flex items-center gap-1 transition-colors disabled:opacity-60 disabled:cursor-default"
                        >
                            {pending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <RefreshCcwIcon className="w-3 h-3" />}
                            {pending ? "Checking" : detail.checked_at ? "Re-verify" : "Verify now"}
                        </button>
                    </div>
                    <div className="mt-0.5 flex items-center gap-1 text-[11px] text-slate-500 min-w-0">
                        {detail.source === "provider" ? (
                            <BadgeCheckIcon className="w-3 h-3 shrink-0 text-sky-600" />
                        ) : null}
                        <span className="truncate" title={detail.checked_at ? fmtAbsolute(detail.checked_at) : undefined}>
                            {source ? source.charAt(0).toUpperCase() + source.slice(1) : "Not checked yet"}
                            {checkSaid ? `, which said ${checkSaid}` : ""}
                            {detail.checked_at ? ` · ${fmtRelative(detail.checked_at)}` : ""}
                        </span>
                    </div>
                    <ul className="mt-0.5 space-y-0.5">
                        <AnimatePresence initial={false}>
                            {detail.reasons.slice(0, 3).map((reason, i) => (
                                <motion.li
                                    key={reason}
                                    initial={{ opacity: 0, x: -6 }}
                                    animate={{ opacity: 1, x: 0 }}
                                    transition={{ delay: i * 0.06 }}
                                    className="text-[11.5px] text-slate-600 leading-snug"
                                >
                                    {reason.charAt(0).toUpperCase() + reason.slice(1)}
                                </motion.li>
                            ))}
                        </AnimatePresence>
                    </ul>
                </div>
            </div>
            <AnimatePresence initial={false}>
                {pending && (
                    <motion.div
                        key="pending"
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: "auto", opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ type: "spring", duration: 0.35, bounce: 0.1 }}
                        className="overflow-hidden"
                    >
                        <div className="border-t border-sky-100 bg-sky-50/70 px-3 py-2 flex items-start gap-2 text-[11.5px] text-sky-800">
                            <Loader2Icon className="w-3 h-3 mt-0.5 shrink-0 animate-spin" />
                            <span className="leading-snug">
                                Re-verifying{runner ? ` with ${runner}` : ""}. The current verdict stands until the result lands, which updates here on its own.
                            </span>
                        </div>
                        {overview?.provider_error && (
                            <div className="border-t border-amber-100 bg-amber-50/70 px-3 py-2 flex items-start gap-2 text-[11.5px] text-amber-800">
                                <AlertTriangleIcon className="w-3 h-3 mt-0.5 shrink-0" />
                                <span className="leading-snug">
                                    {overview.provider_error}{" "}
                                    <Link to="/app/integrations" className="underline underline-offset-2 hover:text-amber-900">
                                        Integrations
                                    </Link>
                                </span>
                            </div>
                        )}
                    </motion.div>
                )}
            </AnimatePresence>
            {detail.evidence.length > 0 && (
                <div className="border-t border-slate-100 divide-y divide-slate-100">
                    {detail.evidence.slice(0, 6).map((e, i) => {
                        const m = EVIDENCE[e.kind] ?? EVIDENCE.bounced_other;
                        const EIcon = m.Icon;
                        return (
                            <motion.div
                                key={`${e.kind}-${e.observed_at}-${i}`}
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                transition={{ delay: 0.15 + i * 0.05 }}
                                className="px-3 py-1.5 flex items-center gap-2 text-[11.5px]"
                            >
                                <EIcon className={cn("w-3 h-3 shrink-0", m.tone)} />
                                <span className="text-slate-700 truncate">{m.label}</span>
                                <span className="ml-auto text-slate-400 shrink-0">{fmtRelative(e.observed_at)}</span>
                            </motion.div>
                        );
                    })}
                </div>
            )}
        </div>
    );
}
