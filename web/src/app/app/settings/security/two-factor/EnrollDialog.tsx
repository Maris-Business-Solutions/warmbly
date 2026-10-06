// The 2FA setup wizard: scan a QR code (or type the key), verify one code from
// the app, then save the recovery codes. The same shape GitHub, Stripe and
// Vercel use, so nobody has to learn it.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { QRCodeSVG } from "qrcode.react";
import toast from "react-hot-toast/headless";
import {
    ArrowLeftIcon,
    CheckIcon,
    ChevronDownIcon,
    CopyIcon,
    KeyRoundIcon,
    Loader2Icon,
    QrCodeIcon,
    ShieldCheckIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { useTwoFactorEnrollStart, useTwoFactorEnrollConfirm } from "@/lib/api/hooks/auth/useTwoFactor";
import type { TwoFactorEnrollStart } from "@/lib/api/client/auth/twoFactor";
import DialogShell, { PrimaryButton, SecondaryButton } from "./DialogShell";
import CodeEntry from "./CodeEntry";
import RecoveryCodesPanel from "./RecoveryCodesPanel";

const STEPS = [
    { key: "scan", label: "Scan" },
    { key: "verify", label: "Verify" },
    { key: "codes", label: "Save codes" },
] as const;

type StepKey = (typeof STEPS)[number]["key"];

const paneVariants = {
    enter: (dir: 1 | -1) => ({ x: dir * 28, opacity: 0 }),
    center: { x: 0, opacity: 1 },
    exit: (dir: 1 | -1) => ({ x: dir * -28, opacity: 0 }),
};

const APPS = ["Google Authenticator", "Microsoft Authenticator", "1Password", "Authy", "Bitwarden"];

export default function EnrollDialog({ onClose, onDone }: { onClose: () => void; onDone: () => void }) {
    const start = useTwoFactorEnrollStart();
    const confirm = useTwoFactorEnrollConfirm();

    const [step, setStep] = React.useState<StepKey>("scan");
    const [direction, setDirection] = React.useState<1 | -1>(1);
    const [info, setInfo] = React.useState<TwoFactorEnrollStart | null>(null);
    const [codeError, setCodeError] = React.useState<string | null>(null);
    const [codes, setCodes] = React.useState<string[]>([]);
    const [saved, setSaved] = React.useState(false);
    const started = React.useRef(false);

    React.useEffect(() => {
        // One secret per open: StrictMode replays this effect, and a second
        // start would replace the pending secret behind the QR on screen.
        if (started.current) return;
        started.current = true;
        start
            .mutateAsync()
            .then(setInfo)
            .catch((e) => {
                toast.error(buildError(e as AppError));
                onClose();
            });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const goTo = (next: StepKey) => {
        const from = STEPS.findIndex((s) => s.key === step);
        const to = STEPS.findIndex((s) => s.key === next);
        setDirection(to > from ? 1 : -1);
        setStep(next);
    };

    const submitCode = async (code: string) => {
        setCodeError(null);
        try {
            const res = await confirm.mutateAsync(code);
            setCodes(res.recovery_codes);
            goTo("codes");
        } catch (e) {
            const err = e as AppError;
            setCodeError(
                err.code === "two_fa_invalid_code"
                    ? "That code didn't match. Wait for a fresh one and try again."
                    : buildError(err),
            );
        }
    };

    // The recovery codes are shown once: no backdrop, Escape or X until the
    // user says they are saved, nor while the code is being checked.
    const locked = step === "codes";
    const stepIndex = STEPS.findIndex((s) => s.key === step);

    return (
        <DialogShell
            title="Set up two-factor authentication"
            icon={<ShieldCheckIcon className="w-3 h-3" />}
            onClose={locked || confirm.isPending ? undefined : onClose}
            width="md"
            footer={
                step === "scan" ? (
                    <>
                        <SecondaryButton onClick={onClose}>Cancel</SecondaryButton>
                        <PrimaryButton className="ml-auto" disabled={!info} onClick={() => goTo("verify")}>
                            Continue
                        </PrimaryButton>
                    </>
                ) : step === "verify" ? (
                    <>
                        <SecondaryButton onClick={() => goTo("scan")} disabled={confirm.isPending}>
                            <ArrowLeftIcon className="w-3.5 h-3.5" /> Back
                        </SecondaryButton>
                        <span className="ml-auto text-[11.5px] text-slate-400">Verifies as soon as the sixth digit lands</span>
                    </>
                ) : (
                    <PrimaryButton
                        className="ml-auto"
                        disabled={!saved}
                        onClick={() => {
                            toast.success("Two-factor authentication is on");
                            onDone();
                        }}
                    >
                        <CheckIcon className="w-3.5 h-3.5" /> Done
                    </PrimaryButton>
                )
            }
        >
            <Stepper step={stepIndex} />
            <AnimatePresence mode="wait" initial={false} custom={direction}>
                <motion.div
                    key={step}
                    custom={direction}
                    variants={paneVariants}
                    initial="enter"
                    animate="center"
                    exit="exit"
                    transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                    className="px-5 py-5"
                >
                    {step === "scan" && <ScanStep info={info} />}
                    {step === "verify" && (
                        <VerifyStep onSubmit={submitCode} pending={confirm.isPending} error={codeError} />
                    )}
                    {step === "codes" && (
                        <div className="space-y-3">
                            <div>
                                <p className="text-[13px] font-medium text-slate-900">Save your recovery codes</p>
                                <p className="text-[12px] text-slate-500 leading-relaxed mt-0.5">
                                    Two-factor authentication is on. If you lose your phone, one of these codes gets
                                    you back in. Each works once.
                                </p>
                            </div>
                            <RecoveryCodesPanel
                                codes={codes}
                                account={info?.account ?? ""}
                                saved={saved}
                                onSavedChange={setSaved}
                            />
                        </div>
                    )}
                </motion.div>
            </AnimatePresence>
        </DialogShell>
    );
}

function Stepper({ step }: { step: number }) {
    return (
        <div className="px-5 h-11 border-b border-slate-100 flex items-center shrink-0 bg-slate-50/40">
            {STEPS.map((s, i) => {
                const active = i === step;
                const done = i < step;
                return (
                    <React.Fragment key={s.key}>
                        <div className="inline-flex items-center gap-2 h-7 pl-1 pr-2 shrink-0" aria-current={active ? "step" : undefined}>
                            <span
                                className={cn(
                                    "size-5 rounded-full inline-flex items-center justify-center text-[10.5px] font-semibold tabular-nums transition-colors",
                                    done
                                        ? "bg-sky-600 text-white"
                                        : active
                                          ? "bg-white text-sky-700 ring-1 ring-inset ring-sky-600"
                                          : "bg-white text-slate-400 ring-1 ring-inset ring-slate-200",
                                )}
                            >
                                {done ? <CheckIcon className="w-3 h-3" strokeWidth={3} /> : i + 1}
                            </span>
                            <span
                                className={cn(
                                    "text-[12px] transition-colors",
                                    active ? "text-slate-900 font-medium" : done ? "text-slate-600" : "text-slate-400",
                                )}
                            >
                                {s.label}
                            </span>
                        </div>
                        {i < STEPS.length - 1 && (
                            <div className={cn("h-px flex-1 min-w-4 mx-1", i < step ? "bg-sky-600/60" : "bg-slate-200")} />
                        )}
                    </React.Fragment>
                );
            })}
        </div>
    );
}

function ScanStep({ info }: { info: TwoFactorEnrollStart | null }) {
    const [manual, setManual] = React.useState(false);
    return (
        <div className="space-y-4">
            <div className="flex flex-col sm:flex-row gap-5">
                <div className="shrink-0 self-center sm:self-start">
                    <div className="ph-no-capture rounded-lg border border-slate-200 bg-white p-2.5 size-[188px] flex items-center justify-center">
                        {info ? (
                            <QRCodeSVG
                                value={info.otpauth_uri}
                                size={164}
                                level="M"
                                marginSize={0}
                                fgColor="#0f172a"
                                bgColor="#ffffff"
                                title={`Scan to add ${info.issuer} to your authenticator`}
                            />
                        ) : (
                            <div className="flex flex-col items-center gap-2 text-[12px] text-slate-400">
                                <Loader2Icon className="w-4 h-4 animate-spin" />
                                Generating…
                            </div>
                        )}
                    </div>
                </div>
                <ol className="flex-1 min-w-0 space-y-3">
                    <Instruction n={1} title="Get an authenticator app">
                        Any TOTP app works.
                        <span className="mt-1.5 flex flex-wrap gap-1">
                            {APPS.map((a) => (
                                <span key={a} className="rounded-sm bg-slate-100 px-1.5 py-0.5 text-[10.5px] text-slate-600">
                                    {a}
                                </span>
                            ))}
                        </span>
                    </Instruction>
                    <Instruction n={2} title="Scan the QR code">
                        In the app, add an account and point the camera at the code.
                    </Instruction>
                    <Instruction n={3} title="Enter the 6-digit code">
                        Press Continue, then type the code the app shows for Warmbly.
                    </Instruction>
                </ol>
            </div>

            <div className="rounded-md border border-slate-200">
                <button
                    type="button"
                    onClick={() => setManual((v) => !v)}
                    aria-expanded={manual}
                    className="w-full h-9 px-3 flex items-center gap-2 text-[12px] text-slate-700 hover:bg-slate-50 transition-colors rounded-md"
                >
                    <KeyRoundIcon className="w-3.5 h-3.5 text-slate-400" />
                    Can&apos;t scan it? Enter the key manually
                    <ChevronDownIcon className={cn("w-3.5 h-3.5 ml-auto text-slate-400 transition-transform", manual && "rotate-180")} />
                </button>
                <AnimatePresence initial={false}>
                    {manual && info && (
                        <motion.div
                            initial={{ height: 0, opacity: 0 }}
                            animate={{ height: "auto", opacity: 1 }}
                            exit={{ height: 0, opacity: 0 }}
                            transition={{ duration: 0.16 }}
                            className="overflow-hidden"
                        >
                            <div className="px-3 pb-3 pt-1 space-y-2.5 border-t border-slate-100">
                                <SecretField secret={info.secret} />
                                <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-[11.5px]">
                                    <Detail label="Account" value={info.account} />
                                    <Detail label="Issuer" value={info.issuer} />
                                    <Detail label="Type" value="Time-based (TOTP)" />
                                    <Detail label="Digits" value={String(info.digits)} />
                                    <Detail label="Interval" value={`${info.period} seconds`} />
                                    <Detail label="Algorithm" value={info.algorithm} />
                                </dl>
                            </div>
                        </motion.div>
                    )}
                </AnimatePresence>
            </div>
        </div>
    );
}

function Instruction({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
    return (
        <li className="flex gap-2.5">
            <span className="size-5 shrink-0 rounded-full bg-slate-100 text-slate-600 text-[10.5px] font-semibold inline-flex items-center justify-center tabular-nums">
                {n}
            </span>
            <div className="min-w-0">
                <div className="text-[12.5px] font-medium text-slate-900 leading-5">{title}</div>
                <div className="text-[11.5px] text-slate-500 leading-relaxed">{children}</div>
            </div>
        </li>
    );
}

function Detail({ label, value }: { label: string; value: string }) {
    return (
        <>
            <dt className="text-slate-400">{label}</dt>
            <dd className="text-slate-700 min-w-0 truncate">{value}</dd>
        </>
    );
}

function SecretField({ secret }: { secret: string }) {
    const [copied, setCopied] = React.useState(false);
    const grouped = secret.match(/.{1,4}/g)?.join(" ") ?? secret;
    return (
        <div>
            <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 mb-1">Setup key</div>
            <button
                type="button"
                onClick={async () => {
                    try {
                        await navigator.clipboard.writeText(secret);
                        setCopied(true);
                        setTimeout(() => setCopied(false), 1600);
                    } catch {
                        toast.error("Couldn't copy. Select the key and copy it by hand.");
                    }
                }}
                className="w-full flex items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-2.5 py-2 font-mono text-[12.5px] tracking-wider text-slate-800 hover:border-slate-300 transition-colors"
                title="Copy setup key"
            >
                <span className="min-w-0 flex-1 text-left break-all" data-ph-mask="">
                    {grouped}
                </span>
                {copied ? (
                    <CheckIcon className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
                ) : (
                    <CopyIcon className="w-3.5 h-3.5 text-slate-400 shrink-0" />
                )}
            </button>
        </div>
    );
}

function VerifyStep({
    onSubmit,
    pending,
    error,
}: {
    onSubmit: (code: string) => void;
    pending: boolean;
    error: string | null;
}) {
    return (
        <div className="max-w-[340px] mx-auto text-center space-y-4 py-2">
            <div className="mx-auto size-11 rounded-xl bg-sky-50 flex items-center justify-center">
                <QrCodeIcon className="w-5 h-5 text-sky-500" />
            </div>
            <div>
                <p className="text-[13px] font-medium text-slate-900">Enter the code from your app</p>
                <p className="text-[12px] text-slate-500 leading-relaxed mt-0.5">
                    Open your authenticator and type the 6-digit code shown next to Warmbly. It changes every 30
                    seconds.
                </p>
            </div>
            <CodeEntry onSubmit={onSubmit} pending={pending} error={error} />
        </div>
    );
}
