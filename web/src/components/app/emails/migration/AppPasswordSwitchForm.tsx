// Moves one mailbox off Google sign-in onto an app password, in place: same
// mailbox, same history. The server checks the password with Gmail first.
import React from "react";
import toast from "react-hot-toast/headless";
import { CheckIcon, ExternalLinkIcon, Loader2Icon } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { SIGNIN_MIGRATION_KEY, useSwitchToAppPassword } from "@/lib/api/hooks/app/emails/useMailboxGrants";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { cn } from "@/lib/utils";
import { SecretInput } from "../import/parts";

const APP_PASSWORD_LENGTH = 16;

export default function AppPasswordSwitchForm({
    mailbox,
    onDirtyChange,
    onDone,
}: {
    mailbox: { id: string; email: string };
    onDirtyChange?: (dirty: boolean) => void;
    onDone?: () => void;
}) {
    const qc = useQueryClient();
    const sw = useSwitchToAppPassword();
    const [password, setPassword] = React.useState("");
    const [error, setError] = React.useState<string | null>(null);
    const cleaned = password.replace(/\s+/g, "");

    React.useEffect(() => {
        onDirtyChange?.(password.trim() !== "");
        return () => onDirtyChange?.(false);
    }, [password, onDirtyChange]);

    async function submit(e: React.FormEvent) {
        e.preventDefault();
        if (sw.isPending) return;
        if (!cleaned) {
            setError("Paste the app password first.");
            return;
        }
        if (cleaned.length !== APP_PASSWORD_LENGTH) {
            setError(`Google app passwords are ${APP_PASSWORD_LENGTH} letters. This looks like something else, maybe the account password.`);
            return;
        }
        setError(null);
        try {
            await sw.mutateAsync({ id: mailbox.id, appPassword: cleaned });
            setPassword("");
            toast.success(`${mailbox.email} now connects with an app password`);
            onDone?.();
        } catch (err) {
            const ae = err as AppError;
            // Already moved, by a teammate or another tab: the list catches up.
            if (ae?.code === "mailbox_not_google_signin") {
                qc.invalidateQueries({ queryKey: SIGNIN_MIGRATION_KEY });
                toast(buildError(ae) || `${mailbox.email} no longer uses Google sign-in.`);
                return;
            }
            setError(buildError(ae));
        }
    }

    return (
        <form onSubmit={(e) => void submit(e)} className="space-y-1.5">
            <div className="flex items-center gap-2">
                <div className="flex-1 min-w-0">
                    <SecretInput
                        value={password}
                        onChange={(v) => {
                            setPassword(v);
                            if (error) setError(null);
                        }}
                        placeholder="xxxx xxxx xxxx xxxx"
                        invalid={!!error}
                        inputClassName="font-mono tracking-wide"
                    />
                </div>
                <button
                    type="submit"
                    aria-disabled={sw.isPending}
                    className={cn(
                        "shrink-0 h-7 px-2.5 rounded-md bg-slate-900 hover:bg-slate-800 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors",
                        sw.isPending && "opacity-60",
                    )}
                >
                    {sw.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <CheckIcon className="w-3 h-3" />}
                    {sw.isPending ? "Checking…" : "Switch"}
                </button>
            </div>
            {error ? (
                <p role="alert" className="text-[11.5px] text-red-700 leading-relaxed">
                    {error}
                </p>
            ) : (
                <p className="text-[11px] text-slate-500 leading-relaxed">
                    Needs 2-Step Verification. Create one at{" "}
                    <a
                        href="https://myaccount.google.com/apppasswords"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-0.5 text-sky-700 underline decoration-sky-300 hover:decoration-sky-600"
                    >
                        myaccount.google.com/apppasswords
                        <ExternalLinkIcon className="w-2.5 h-2.5" />
                    </a>{" "}
                    while signed in as {mailbox.email}. History, campaigns and warmup stay.
                </p>
            )}
        </form>
    );
}
