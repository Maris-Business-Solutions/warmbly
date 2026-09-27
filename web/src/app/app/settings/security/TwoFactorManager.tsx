// Two-factor (TOTP) status and management. Setup runs in EnrollDialog (scan a
// QR code, verify a code, save recovery codes); once on, this shows when it
// was enabled and how many recovery codes are left, with regenerate and
// disable actions.

import React from "react";
import { AnimatePresence } from "framer-motion";
import { KeyRoundIcon, RefreshCwIcon, ShieldCheckIcon, SmartphoneIcon, TriangleAlertIcon } from "lucide-react";
import { useTwoFactorStatus } from "@/lib/api/hooks/auth/useTwoFactor";
import { useUserProfile } from "@/hooks/context/user";
import { cn } from "@/lib/utils";
import { Row, Section } from "../_components/SectionShell";
import EnrollDialog from "./two-factor/EnrollDialog";
import RegenerateDialog from "./two-factor/RegenerateDialog";
import DisableDialog from "./two-factor/DisableDialog";

type Dialog = "enroll" | "regenerate" | "disable" | null;

// Below this many unused codes the row turns amber and asks for a new set.
const LOW_CODES = 3;

const fmt = (d: string | Date) =>
    new Date(d).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });

export default function TwoFactorManager() {
    const { data: status, isLoading, isError, refetch, isFetching } = useTwoFactorStatus();
    const { user } = useUserProfile();
    const [dialog, setDialog] = React.useState<Dialog>(null);
    const close = React.useCallback(() => setDialog(null), []);

    const enabled = !!status?.enabled;
    const remaining = status?.recovery_codes_remaining ?? 0;
    const total = status?.recovery_codes_total ?? 0;
    const low = enabled && remaining <= LOW_CODES;

    return (
        <Section
            eyebrow="Two-factor authentication"
            description="A rotating code from an authenticator app on every password sign-in, so a stolen password alone is not enough."
        >
            {isError && !status ? (
                <Row
                    label="Couldn't load your two-factor status"
                    description="Nothing has changed on your account. Try again in a moment."
                >
                    <button
                        type="button"
                        onClick={() => void refetch()}
                        disabled={isFetching}
                        className="h-7 px-2.5 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-slate-300 hover:text-slate-900 transition-colors inline-flex items-center gap-1.5 disabled:opacity-50"
                    >
                        <RefreshCwIcon className={cn("w-3 h-3", isFetching && "animate-spin")} />
                        Retry
                    </button>
                </Row>
            ) : isLoading ? (
                <div className="space-y-2" aria-busy="true">
                    <div className="h-4 w-48 rounded bg-slate-100 animate-pulse" />
                    <div className="h-3 w-72 rounded bg-slate-100 animate-pulse" />
                </div>
            ) : enabled ? (
                <div className="rounded-md border border-slate-200 divide-y divide-slate-200 bg-white">
                    <Line
                        icon={<ShieldCheckIcon className="w-4 h-4 text-emerald-600" />}
                        tone="ok"
                        title={
                            <span className="inline-flex items-center gap-2">
                                Authenticator app
                                <span className="text-[10px] uppercase tracking-[0.08em] font-medium rounded-sm px-1 bg-emerald-50 text-emerald-700">
                                    On
                                </span>
                            </span>
                        }
                        description={
                            status?.confirmed_at
                                ? `Enabled ${fmt(status.confirmed_at)}. Time-based codes, 30-second interval.`
                                : "Time-based codes, 30-second interval."
                        }
                    >
                        <button
                            type="button"
                            onClick={() => setDialog("disable")}
                            className="h-7 px-2.5 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-rose-300 hover:text-rose-600 transition-colors"
                        >
                            Turn off
                        </button>
                    </Line>
                    <Line
                        icon={
                            low ? (
                                <TriangleAlertIcon className="w-4 h-4 text-amber-600" />
                            ) : (
                                <KeyRoundIcon className="w-4 h-4 text-sky-500" />
                            )
                        }
                        tone={low ? "warn" : "info"}
                        title={
                            <span className="inline-flex items-center gap-2">
                                Recovery codes
                                <span
                                    className={cn(
                                        "text-[10px] uppercase tracking-[0.08em] font-medium rounded-sm px-1 tabular-nums",
                                        low ? "bg-amber-50 text-amber-700" : "bg-slate-100 text-slate-500",
                                    )}
                                >
                                    {remaining} of {total} left
                                </span>
                            </span>
                        }
                        description={
                            remaining === 0
                                ? "None left. Generate a new set now, or losing your phone locks you out."
                                : low
                                  ? "Running low. Generate a new set and store it with your password manager."
                                  : "Each signs you in once if you lose your authenticator. Generating a new set replaces every existing code."
                        }
                    >
                        <button
                            type="button"
                            onClick={() => setDialog("regenerate")}
                            className={cn(
                                "h-7 px-2.5 rounded-md border text-[12px] transition-colors inline-flex items-center gap-1.5",
                                low
                                    ? "border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100"
                                    : "border-slate-200 text-slate-700 hover:border-slate-300 hover:text-slate-900",
                            )}
                        >
                            <RefreshCwIcon className="w-3 h-3" />
                            {low ? "Generate new codes" : "Regenerate"}
                        </button>
                    </Line>
                </div>
            ) : (
                <Row
                    label={
                        <span className="inline-flex items-center gap-2">
                            Authenticator app
                            <span className="text-[10px] uppercase tracking-[0.08em] font-medium rounded-sm px-1 bg-slate-100 text-slate-500">
                                Off
                            </span>
                        </span>
                    }
                    description="Scan a QR code with Google Authenticator, 1Password, Authy or any TOTP app. Takes about a minute."
                >
                    <button
                        type="button"
                        onClick={() => setDialog("enroll")}
                        className="h-7 px-2.5 rounded-md bg-sky-600 text-white text-[12px] font-medium hover:bg-sky-700 transition-colors inline-flex items-center gap-1.5"
                    >
                        <SmartphoneIcon className="w-3.5 h-3.5" />
                        Set up
                    </button>
                </Row>
            )}

            <AnimatePresence>
                {dialog === "enroll" && <EnrollDialog key="enroll" onClose={close} onDone={close} />}
                {dialog === "regenerate" && (
                    <RegenerateDialog key="regenerate" account={user.email} remaining={remaining} onClose={close} />
                )}
                {dialog === "disable" && <DisableDialog key="disable" onClose={close} />}
            </AnimatePresence>
        </Section>
    );
}

function Line({
    icon,
    tone,
    title,
    description,
    children,
}: {
    icon: React.ReactNode;
    tone: "ok" | "info" | "warn";
    title: React.ReactNode;
    description: string;
    children: React.ReactNode;
}) {
    return (
        <div className="flex items-center gap-3 px-3 py-2.5">
            <div
                className={cn(
                    "w-8 h-8 rounded-lg flex items-center justify-center shrink-0",
                    tone === "ok" ? "bg-emerald-50" : tone === "warn" ? "bg-amber-50" : "bg-sky-50",
                )}
            >
                {icon}
            </div>
            <div className="min-w-0 flex-1">
                <div className="text-[12.5px] font-medium text-slate-900 leading-tight">{title}</div>
                <div className="text-[11.5px] text-slate-500 leading-snug mt-0.5">{description}</div>
            </div>
            <div className="shrink-0">{children}</div>
        </div>
    );
}
