import { DashboardImage } from "@/components/ui/dashboard-image";

// app.warmbly.com/connect?code=XXXX-XXXX — a signed-in member approves a
// self-hosted instance's request to warm its mailboxes in this workspace.
// Standalone on the auth screen's sky: enter code, review, done.

import React from "react";
import { Link, Navigate } from "@tanstack/react-router";
import { useSearchParams } from "@/hooks/useSearchParams";
import { AnimatePresence, motion } from "framer-motion";
import { REGEXP_ONLY_DIGITS_AND_CHARS } from "input-otp";
import toast from "react-hot-toast/headless";
import {
    ArrowLeftIcon,
    ArrowRightIcon,
    BuildingIcon,
    CheckIcon,
    CloudIcon,
    ExternalLinkIcon,
    FlameIcon,
    Loader2Icon,
    LockIcon,
    ServerIcon,
    XIcon,
} from "lucide-react";
import { InputOTP, InputOTPGroup, InputOTPSlot } from "@/components/ui/input-otp";
import BrandMark from "@/components/shared/BrandMark";
import ReauthModal from "@/components/app/modals/ReauthModal";
import getToken from "@/lib/helper/getToken";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import type Organization from "@/lib/api/models/app/organizations/Organization";
import useOrganizations from "@/lib/api/hooks/app/organizations/useOrganizations";
import { useApprovePoolLinkCode, useDenyPoolLinkCode, usePoolLinkCode } from "@/lib/api/hooks/app/cloudlink/useCloudLink";

const CODE_LENGTH = 8;

function clean(raw: string): string {
    return raw.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, CODE_LENGTH);
}

function dashed(code: string): string {
    return code.length > 4 ? `${code.slice(0, 4)}-${code.slice(4)}` : code;
}

const slide = {
    enter: (dir: number) => ({ opacity: 0, x: dir > 0 ? 28 : -28 }),
    center: { opacity: 1, x: 0 },
    exit: (dir: number) => ({ opacity: 0, x: dir > 0 ? -28 : 28 }),
};
const slideTransition = { duration: 0.28, ease: [0.16, 1, 0.3, 1] as const };

export default function ConnectPage() {
    if (!getToken()) {
        const next = window.location.pathname + window.location.search;
        return <Navigate to="/auth/login" search={{ next }} replace />;
    }
    // Outside the /app layout, so approving (which needs a fresh confirmation) brings its own prompt.
    return (
        <>
            <ConnectInner />
            <ReauthModal />
        </>
    );
}

function ConnectInner() {
    const [params] = useSearchParams();
    const [code, setCode] = React.useState(() => clean(params.get("code") ?? ""));
    const complete = code.length === CODE_LENGTH;
    const lookup = usePoolLinkCode(dashed(code));
    const orgs = useOrganizations();
    const approve = useApprovePoolLinkCode();
    const deny = useDenyPoolLinkCode();
    const [orgId, setOrgId] = React.useState("");
    const [outcome, setOutcome] = React.useState<"approved" | "denied" | null>(null);
    const [dir, setDir] = React.useState(1);

    React.useEffect(() => {
        if (!orgId && orgs.data && orgs.data.length > 0) setOrgId(orgs.data[0].id);
    }, [orgs.data, orgId]);

    const info = complete ? lookup.data : undefined;
    const step: "code" | "review" | "done" = outcome ? "done" : info ? "review" : "code";

    const reset = () => {
        setDir(-1);
        setCode("");
        setOutcome(null);
    };

    const doApprove = async () => {
        try {
            setDir(1);
            await approve.mutateAsync({ code: dashed(code), organizationId: orgId });
            setOutcome("approved");
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    };
    const doDeny = async () => {
        try {
            setDir(1);
            await deny.mutateAsync(dashed(code));
            setOutcome("denied");
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    };

    return (
        <div className="relative min-h-dvh w-full overflow-hidden flex flex-col items-center justify-center px-4 py-8 sm:py-10">
            <div className="absolute inset-0" aria-hidden="true">
                <div className="sky-base" />
                <div className="sky-breathe" />
                <div className="sun-glow" />
                <DashboardImage src="/backdrops/cloud-3.webp" alt="" decoding="async" className="cloud-drift cloud-1 absolute select-none" style={{ top: "6%", left: "-10%", width: 360, opacity: 0.55, height: "auto" }} />
                <DashboardImage src="/backdrops/cloud-4.webp" alt="" decoding="async" className="cloud-drift cloud-2 absolute select-none" style={{ bottom: "8%", right: "-8%", width: 320, opacity: 0.5, height: "auto" }} />
                <DashboardImage src="/backdrops/cloud-1.webp" alt="" decoding="async" className="cloud-drift cloud-1 absolute select-none" style={{ top: "44%", right: "14%", width: 220, opacity: 0.35, height: "auto" }} />
            </div>

            <div className="relative z-10 w-full max-w-[560px]">
                <BrandMark className="mb-5 flex w-fit items-center gap-2.5 mx-auto" />

                <motion.div
                    initial={{ y: 14, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
                    className="animate-card-float rounded-3xl border border-slate-200 bg-white shadow-[0_1px_2px_rgba(15,23,42,0.04),0_30px_70px_-32px_rgba(15,23,42,0.32)] overflow-hidden"
                >
                    <Steps current={step} />

                    <div className="px-6 pb-7 pt-2 sm:px-10 sm:pb-9 overflow-hidden">
                        <AnimatePresence mode="wait" initial={false} custom={dir}>
                            {step === "code" && (
                                <motion.div key="code" custom={dir} variants={slide} initial="enter" animate="center" exit="exit" transition={slideTransition}>
                                    <CodeStep code={code} setCode={setCode} loading={complete && lookup.isLoading} error={complete && lookup.isError ? (lookup.error as unknown as AppError) : null} onRetry={reset} />
                                </motion.div>
                            )}
                            {step === "review" && info && (
                                <motion.div key="review" custom={dir} variants={slide} initial="enter" animate="center" exit="exit" transition={slideTransition}>
                                    <ReviewStep
                                        info={info}
                                        orgs={orgs.data ?? []}
                                        orgsLoading={orgs.isLoading}
                                        orgId={orgId}
                                        setOrgId={setOrgId}
                                        busy={approve.isPending || deny.isPending}
                                        approving={approve.isPending}
                                        onApprove={doApprove}
                                        onDeny={doDeny}
                                        onBack={reset}
                                    />
                                </motion.div>
                            )}
                            {step === "done" && info && (
                                <motion.div key="done" custom={dir} variants={slide} initial="enter" animate="center" exit="exit" transition={slideTransition}>
                                    <DoneStep approved={outcome === "approved"} instanceName={info.instance_name} instanceUrl={info.instance_url} orgName={orgs.data?.find((o) => o.id === orgId)?.name} onAnother={reset} />
                                </motion.div>
                            )}
                        </AnimatePresence>
                    </div>
                </motion.div>

                <div className="theme-solid mt-5 flex items-center justify-center gap-3 text-[12px] text-white/70">
                    <Link to="/app/emails" className="hover:text-white transition-colors">Back to dashboard</Link>
                    <span className="text-white/40">·</span>
                    <a href="https://docs.warmbly.com/guides/warmbly-cloud/" target="_blank" rel="noreferrer" className="hover:text-white transition-colors">How linking works</a>
                </div>
            </div>
        </div>
    );
}

const STEPS: { key: "code" | "review" | "done"; label: string }[] = [
    { key: "code", label: "Code" },
    { key: "review", label: "Review" },
    { key: "done", label: "Linked" },
];

function Steps({ current }: { current: "code" | "review" | "done" }) {
    const idx = STEPS.findIndex((s) => s.key === current);
    return (
        <div className="px-6 sm:px-10 pt-7 pb-5 flex items-center gap-2">
            {STEPS.map((s, i) => {
                const state = i < idx ? "done" : i === idx ? "current" : "todo";
                return (
                    <React.Fragment key={s.key}>
                        <div className="flex items-center gap-2">
                            <motion.span
                                animate={{
                                    backgroundColor: state === "todo" ? "var(--color-slate-100)" : "#0284c7",
                                    color: state === "todo" ? "var(--color-slate-400)" : "#ffffff",
                                }}
                                className="size-6 rounded-full inline-flex items-center justify-center text-[11px] font-semibold"
                            >
                                {state === "done" ? <CheckIcon className="w-3 h-3" /> : i + 1}
                            </motion.span>
                            <span className={`text-[12px] font-medium ${state === "todo" ? "text-slate-400" : "text-slate-900"}`}>{s.label}</span>
                        </div>
                        {i < STEPS.length - 1 && (
                            <span className="relative flex-1 h-px bg-slate-200 overflow-hidden rounded-full">
                                <motion.span animate={{ width: i < idx ? "100%" : "0%" }} transition={{ duration: 0.4 }} className="absolute inset-y-0 left-0 bg-sky-600" />
                            </span>
                        )}
                    </React.Fragment>
                );
            })}
        </div>
    );
}

function CodeStep({ code, setCode, loading, error, onRetry }: { code: string; setCode: (c: string) => void; loading: boolean; error: AppError | null; onRetry: () => void }) {
    return (
        <div>
            <div className="text-center">
                <span className="inline-flex items-center gap-1.5 h-6 px-2.5 rounded-full bg-sky-50 text-sky-700 text-[11px] font-medium">
                    <CloudIcon className="w-3 h-3" /> Warmbly Cloud
                </span>
                <h1 className="mt-4 text-[24px] sm:text-[28px] font-semibold tracking-[-0.03em] leading-[1.1] text-slate-900">Link your self-hosted instance</h1>
                <p className="mt-2.5 text-[13.5px] text-slate-500 leading-relaxed max-w-md mx-auto">
                    Enter the eight character code your instance is showing. Its mailboxes then warm in the Warmbly pool while everything else stays on your server.
                </p>
            </div>

            <div className="mt-8 flex justify-center">
                <InputOTP maxLength={CODE_LENGTH} value={code} onChange={(v) => setCode(clean(v))} pattern={REGEXP_ONLY_DIGITS_AND_CHARS} pasteTransformer={clean} autoFocus containerClassName="gap-1.5 sm:gap-2" disabled={loading}>
                    <InputOTPGroup className="gap-1.5 sm:gap-2">
                        {[0, 1, 2, 3].map((i) => (
                            <Slot key={i} index={i} />
                        ))}
                    </InputOTPGroup>
                    <span className="w-3 h-px bg-slate-300 mx-0.5 sm:mx-1" aria-hidden="true" />
                    <InputOTPGroup className="gap-1.5 sm:gap-2">
                        {[4, 5, 6, 7].map((i) => (
                            <Slot key={i} index={i} />
                        ))}
                    </InputOTPGroup>
                </InputOTP>
            </div>

            <div className="mt-5 min-h-[44px] flex items-center justify-center">
                <AnimatePresence mode="wait" initial={false}>
                    {loading && (
                        <motion.p key="loading" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="inline-flex items-center gap-2 text-[12.5px] text-slate-500">
                            <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> Looking up your instance
                        </motion.p>
                    )}
                    {error && !loading && (
                        <motion.div key="error" initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }} className="w-full rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-center">
                            <p className="text-[13px] font-medium text-rose-700">{error.message || "That code is unknown or has expired."}</p>
                            <p className="mt-0.5 text-[12px] text-rose-600/80">
                                Press Connect again on your instance for a fresh one, then{" "}
                                <button type="button" onClick={onRetry} className="font-medium underline underline-offset-2 hover:text-rose-800">
                                    enter it here
                                </button>
                                .
                            </p>
                        </motion.div>
                    )}
                    {!loading && !error && (
                        <motion.p key="hint" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="text-[12px] text-slate-400 text-center">
                            You can paste the whole code. It is on your instance under Settings, Warmbly Cloud.
                        </motion.p>
                    )}
                </AnimatePresence>
            </div>
        </div>
    );
}

function Slot({ index }: { index: number }) {
    return (
        <InputOTPSlot
            index={index}
            className="!w-10 !h-12 sm:!w-12 sm:!h-14 !rounded-lg !border !border-slate-200 !shadow-none first:!rounded-lg last:!rounded-lg text-[20px] font-semibold font-mono text-slate-900 data-[active=true]:!border-sky-400 data-[active=true]:!ring-sky-400/15"
        />
    );
}

function ReviewStep({
    info,
    orgs,
    orgsLoading,
    orgId,
    setOrgId,
    busy,
    approving,
    onApprove,
    onDeny,
    onBack,
}: {
    info: { instance_name: string; instance_url: string; instance_version: string; status: string };
    orgs: Organization[];
    orgsLoading: boolean;
    orgId: string;
    setOrgId: (id: string) => void;
    busy: boolean;
    approving: boolean;
    onApprove: () => void;
    onDeny: () => void;
    onBack: () => void;
}) {
    const pending = info.status === "pending";
    const host = (() => {
        try {
            return info.instance_url ? new URL(info.instance_url).host : "";
        } catch {
            return info.instance_url;
        }
    })();

    return (
        <div>
            <button type="button" onClick={onBack} className="inline-flex items-center gap-1 text-[12px] text-slate-500 hover:text-slate-900 transition-colors">
                <ArrowLeftIcon className="w-3.5 h-3.5" /> Different code
            </button>

            <h1 className="mt-3 text-[22px] sm:text-[26px] font-semibold tracking-[-0.03em] leading-[1.1] text-slate-900">
                {pending ? "This instance wants to warm its mailboxes here" : "This code was already used"}
            </h1>

            <div className="mt-5 flex items-center gap-4 rounded-xl border border-slate-200 bg-gradient-to-b from-sky-50/60 to-white px-4 py-4">
                <span className="size-11 rounded-lg bg-slate-900 text-white inline-flex items-center justify-center shrink-0">
                    <ServerIcon className="w-5 h-5" />
                </span>
                <div className="min-w-0 flex-1">
                    <p className="text-[15px] font-semibold text-slate-900 truncate">{info.instance_name || "Self-hosted instance"}</p>
                    <p className="text-[12px] text-slate-500 truncate">
                        {host || "Address not shared"}
                        {info.instance_version && <span className="text-slate-400"> · v{info.instance_version}</span>}
                    </p>
                </div>
                <Flow />
            </div>

            {!pending ? (
                <div className="mt-5">
                    <p className="text-[13px] text-slate-500 leading-relaxed">
                        {info.status === "denied"
                            ? "The request was declined. Start a new one from the instance if you changed your mind."
                            : "It is linked already. If the instance is still waiting, start a new request from its settings."}
                    </p>
                    <div className="mt-5 flex items-center gap-2">
                        <Link to="/app/settings/warmbly-cloud" className="h-10 px-4 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[13px] font-medium inline-flex items-center gap-1.5 transition-colors">
                            Linked instances <ArrowRightIcon className="w-3.5 h-3.5" />
                        </Link>
                    </div>
                </div>
            ) : (
                <>
                    <div className="mt-6">
                        <p className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Warm in workspace</p>
                        <div className="mt-2 space-y-1.5">
                            {orgsLoading && (
                                <div className="h-12 rounded-lg border border-slate-200 flex items-center justify-center text-slate-400">
                                    <Loader2Icon className="w-4 h-4 animate-spin" />
                                </div>
                            )}
                            {orgs.map((o) => {
                                const on = o.id === orgId;
                                return (
                                    <button
                                        key={o.id}
                                        type="button"
                                        onClick={() => setOrgId(o.id)}
                                        className={`w-full flex items-center gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors ${
                                            on ? "border-sky-400 bg-sky-50/60 ring-2 ring-sky-100" : "border-slate-200 hover:border-slate-300"
                                        }`}
                                    >
                                        <span className={`size-8 rounded-md inline-flex items-center justify-center shrink-0 overflow-hidden ${on ? "bg-sky-600 text-white" : "bg-slate-100 text-slate-600"}`}>
                                            {o.avatar ? <DashboardImage src={o.avatar} alt="" className="size-full object-cover" /> : <BuildingIcon className="w-4 h-4" />}
                                        </span>
                                        <span className="min-w-0 flex-1">
                                            <span className="block text-[13.5px] font-medium text-slate-900 truncate">{o.name}</span>
                                            <span className="block text-[11.5px] text-slate-500 capitalize">{o.role}</span>
                                        </span>
                                        <span className={`size-4 rounded-full border inline-flex items-center justify-center ${on ? "border-sky-600 bg-sky-600 text-white" : "border-slate-300"}`}>
                                            {on && <CheckIcon className="w-2.5 h-2.5" />}
                                        </span>
                                    </button>
                                );
                            })}
                            {!orgsLoading && orgs.length === 0 && <p className="text-[12.5px] text-slate-500">You are not a member of any workspace yet.</p>}
                        </div>
                    </div>

                    <ul className="mt-5 grid sm:grid-cols-2 gap-2.5">
                        <Perm icon={FlameIcon} title="Can do" body="Enroll its mailboxes in this workspace's warmup pool and read their warmup health." />
                        <Perm icon={LockIcon} title="Cannot do" body="See campaigns, contacts or the inbox here. Unlink it any time from Settings." />
                    </ul>

                    <div className="mt-6 flex items-center gap-2">
                        <button
                            type="button"
                            onClick={onDeny}
                            disabled={busy}
                            className="h-10 px-4 rounded-md border border-slate-200 hover:border-slate-300 text-[13px] text-slate-700 inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                        >
                            <XIcon className="w-3.5 h-3.5" /> Decline
                        </button>
                        <button
                            type="button"
                            onClick={onApprove}
                            disabled={!orgId || busy}
                            className="flex-1 h-10 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[13.5px] font-medium inline-flex items-center justify-center gap-1.5 transition-colors disabled:opacity-60"
                        >
                            {approving ? <Loader2Icon className="w-4 h-4 animate-spin" /> : <CheckIcon className="w-4 h-4" />}
                            Link instance
                        </button>
                    </div>
                </>
            )}
        </div>
    );
}

function Perm({ icon: Icon, title, body }: { icon: React.ComponentType<{ className?: string }>; title: string; body: string }) {
    return (
        <li className="rounded-lg border border-slate-200 px-3 py-2.5 flex items-start gap-2.5">
            <Icon className="w-4 h-4 mt-0.5 text-sky-600 shrink-0" />
            <span>
                <span className="block text-[12px] font-semibold text-slate-900">{title}</span>
                <span className="block text-[12px] text-slate-500 leading-relaxed">{body}</span>
            </span>
        </li>
    );
}

// Tiny instance → cloud diagram with mail dots in motion.
function Flow() {
    return (
        <div className="hidden sm:flex items-center gap-1 shrink-0">
            <span className="relative w-14 h-px bg-slate-200">
                {[0, 1].map((i) => (
                    <motion.span
                        key={i}
                        className="absolute top-1/2 -translate-y-1/2 size-1.5 rounded-full bg-sky-500"
                        initial={{ left: 0, opacity: 0 }}
                        animate={{ left: ["0%", "100%"], opacity: [0, 1, 1, 0] }}
                        transition={{ duration: 1.8, delay: i * 0.9, repeat: Infinity, ease: "easeInOut" }}
                    />
                ))}
            </span>
            <span className="size-9 rounded-lg bg-sky-600 text-white inline-flex items-center justify-center">
                <CloudIcon className="w-4 h-4" />
            </span>
        </div>
    );
}

function DoneStep({ approved, instanceName, instanceUrl, orgName, onAnother }: { approved: boolean; instanceName: string; instanceUrl: string; orgName?: string; onAnother: () => void }) {
    return (
        <div className="flex flex-col items-center text-center py-2">
            <motion.span
                initial={{ scale: 0.5, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                transition={{ type: "spring", stiffness: 380, damping: 20, delay: 0.1 }}
                className={`size-16 rounded-full inline-flex items-center justify-center ${approved ? "bg-emerald-50 text-emerald-600" : "bg-slate-100 text-slate-500"}`}
            >
                {approved ? <CheckIcon className="w-8 h-8" /> : <XIcon className="w-8 h-8" />}
            </motion.span>
            <h1 className="mt-5 text-[24px] sm:text-[28px] font-semibold tracking-[-0.03em] leading-[1.1] text-slate-900">
                {approved ? "Instance linked" : "Request declined"}
            </h1>
            <p className="mt-2.5 text-[13.5px] text-slate-500 leading-relaxed max-w-sm">
                {approved ? (
                    <>
                        <span className="font-medium text-slate-700">{instanceName || "Your instance"}</span> picks this up on its own within a few seconds
                        {orgName ? (
                            <>
                                {" "}
                                and can now warm mailboxes in <span className="font-medium text-slate-700">{orgName}</span>.
                            </>
                        ) : (
                            "."
                        )}{" "}
                        Go back to it to choose which mailboxes to warm.
                    </>
                ) : (
                    "Nothing was linked. The instance will show that the request was declined."
                )}
            </p>
            <div className="mt-7 flex flex-wrap items-center justify-center gap-2">
                {approved && instanceUrl && (
                    <a href={instanceUrl} className="h-10 px-4 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[13.5px] font-medium inline-flex items-center gap-1.5 transition-colors">
                        Back to {instanceName || "your instance"} <ExternalLinkIcon className="w-3.5 h-3.5" />
                    </a>
                )}
                <Link
                    to="/app/settings/warmbly-cloud"
                    className={`h-10 px-4 rounded-md text-[13.5px] font-medium inline-flex items-center gap-1.5 transition-colors ${
                        approved && instanceUrl ? "border border-slate-200 hover:border-slate-300 text-slate-800" : "bg-sky-600 hover:bg-sky-700 text-white"
                    }`}
                >
                    Linked instances <ArrowRightIcon className="w-3.5 h-3.5" />
                </Link>
            </div>
            <button type="button" onClick={onAnother} className="mt-5 text-[12px] text-slate-500 hover:text-slate-900 transition-colors">
                Link another instance
            </button>
        </div>
    );
}
