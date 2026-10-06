// Turning 2FA off needs proof of possession (a current authenticator or
// recovery code) and an explicit button: no auto-submit on the sixth digit.

import React from "react";
import toast from "react-hot-toast/headless";
import { Loader2Icon, ShieldOffIcon, TriangleAlertIcon } from "lucide-react";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { useTwoFactorDisable } from "@/lib/api/hooks/auth/useTwoFactor";
import DialogShell, { SecondaryButton } from "./DialogShell";
import CodeEntry from "./CodeEntry";

export default function DisableDialog({ onClose }: { onClose: () => void }) {
    const disable = useTwoFactorDisable();
    const [code, setCode] = React.useState("");
    const [error, setError] = React.useState<string | null>(null);

    const ready = code.length === 6 || code.includes("-");

    const submit = async (c: string) => {
        if (disable.isPending) return;
        setError(null);
        try {
            await disable.mutateAsync(c.trim());
            toast.success("Two-factor authentication is off");
            onClose();
        } catch (e) {
            const err = e as AppError;
            setError(err.code === "two_fa_invalid_code" ? "That code didn't match. Try again." : buildError(err));
        }
    };

    return (
        <DialogShell
            title="Turn off two-factor authentication"
            icon={<ShieldOffIcon className="w-3 h-3" />}
            onClose={disable.isPending ? undefined : onClose}
            footer={
                <>
                    <SecondaryButton onClick={onClose} disabled={disable.isPending}>
                        Keep it on
                    </SecondaryButton>
                    <button
                        type="button"
                        onClick={() => submit(code)}
                        disabled={!ready || disable.isPending}
                        className="ml-auto h-8 px-3 rounded-md bg-rose-600 text-white text-[12.5px] font-medium hover:bg-rose-700 transition-colors inline-flex items-center justify-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {disable.isPending && <Loader2Icon className="w-3.5 h-3.5 animate-spin" />}
                        Turn off
                    </button>
                </>
            }
        >
            <div className="px-5 py-5 space-y-4">
                <div className="flex gap-2.5 rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5">
                    <TriangleAlertIcon className="w-4 h-4 text-amber-600 shrink-0 mt-px" />
                    <p className="text-[12px] text-amber-900 leading-relaxed">
                        Your account will be protected by your password and the emailed code only. Your recovery
                        codes stop working, and setting 2FA up again issues a new secret.
                    </p>
                </div>
                <p className="text-[12px] text-slate-500">Confirm with a current authenticator code or a recovery code.</p>
                <CodeEntry
                    onSubmit={submit}
                    pending={disable.isPending}
                    error={error}
                    allowRecovery
                    autoSubmit={false}
                    onChange={setCode}
                />
            </div>
        </DialogShell>
    );
}
