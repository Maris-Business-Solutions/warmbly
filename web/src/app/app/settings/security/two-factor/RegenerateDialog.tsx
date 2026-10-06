// Issue a fresh set of recovery codes. Proves possession with a current code
// first; the old codes stop working the moment the new ones are shown.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import toast from "react-hot-toast/headless";
import { CheckIcon, KeyRoundIcon } from "lucide-react";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { useTwoFactorRegenerateRecoveryCodes } from "@/lib/api/hooks/auth/useTwoFactor";
import DialogShell, { PrimaryButton, SecondaryButton } from "./DialogShell";
import CodeEntry from "./CodeEntry";
import RecoveryCodesPanel from "./RecoveryCodesPanel";

export default function RegenerateDialog({
    account,
    remaining,
    onClose,
}: {
    account: string;
    remaining: number;
    onClose: () => void;
}) {
    const regenerate = useTwoFactorRegenerateRecoveryCodes();
    const [codes, setCodes] = React.useState<string[] | null>(null);
    const [error, setError] = React.useState<string | null>(null);
    const [saved, setSaved] = React.useState(false);

    const submit = async (code: string) => {
        setError(null);
        try {
            const res = await regenerate.mutateAsync(code);
            setCodes(res.recovery_codes);
        } catch (e) {
            const err = e as AppError;
            setError(err.code === "two_fa_invalid_code" ? "That code didn't match. Try again." : buildError(err));
        }
    };

    const locked = codes !== null;

    return (
        <DialogShell
            title={locked ? "Your new recovery codes" : "Generate new recovery codes"}
            icon={<KeyRoundIcon className="w-3 h-3" />}
            onClose={locked || regenerate.isPending ? undefined : onClose}
            footer={
                locked ? (
                    <PrimaryButton
                        className="ml-auto"
                        disabled={!saved}
                        onClick={() => {
                            toast.success("Recovery codes replaced");
                            onClose();
                        }}
                    >
                        <CheckIcon className="w-3.5 h-3.5" /> Done
                    </PrimaryButton>
                ) : (
                    <SecondaryButton onClick={onClose} disabled={regenerate.isPending}>
                        Cancel
                    </SecondaryButton>
                )
            }
        >
            <AnimatePresence mode="wait" initial={false}>
                {locked ? (
                    <motion.div
                        key="codes"
                        initial={{ x: 28, opacity: 0 }}
                        animate={{ x: 0, opacity: 1 }}
                        exit={{ x: -28, opacity: 0 }}
                        transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                        className="px-5 py-5 space-y-3"
                    >
                        <p className="text-[12px] text-slate-500 leading-relaxed">
                            Your previous codes no longer work. Replace any copy you kept with this set.
                        </p>
                        <RecoveryCodesPanel codes={codes} account={account} saved={saved} onSavedChange={setSaved} />
                    </motion.div>
                ) : (
                    <motion.div
                        key="verify"
                        initial={{ x: 28, opacity: 0 }}
                        animate={{ x: 0, opacity: 1 }}
                        exit={{ x: -28, opacity: 0 }}
                        transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                        className="px-5 py-5 space-y-4"
                    >
                        <p className="text-[12px] text-slate-500 leading-relaxed">
                            {remaining === 0
                                ? "You have no recovery codes left. "
                                : `You have ${remaining} unused ${remaining === 1 ? "code" : "codes"}. `}
                            Generating a new set invalidates every existing code. Confirm with your authenticator
                            first.
                        </p>
                        <CodeEntry onSubmit={submit} pending={regenerate.isPending} error={error} allowRecovery />
                    </motion.div>
                )}
            </AnimatePresence>
        </DialogShell>
    );
}
