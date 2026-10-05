import type React from "react";
import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSearchParams } from "@/hooks/useSearchParams";
import useRegisterConfirm from "@/lib/api/hooks/auth/useRegisterConfirm";
import toast from "react-hot-toast";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export function useRegisterConfirmForm() {
    // Query params, not path params. See useLoginConfirmForm.
    const [params] = useSearchParams();
    const navigate = useNavigate();
    const registerConfirm = useRegisterConfirm();

    const mail = params.get("to") ?? "";
    const session = params.get("session") ?? "";
    const [otp, setOtp] = useState<string[]>(Array(6).fill(""));
    const [pending, setPending] = useState(false);

    const submit = async () => {
        setPending(true);
        try {
            await toast.promise(
                registerConfirm.mutateAsync({ session, code: otp.map(v => v || "0").join("") }),
                { loading: "Loading...", success: "Account successfully created.", error: (err: AppError) => buildError(err) }
            );
            navigate({ to: "/auth/login", search: { action: "0" } });
        } finally { setPending(false); }
    };

    // No captcha here: the start step already spent one, and the signed,
    // single-use session it issued is the proof. A Turnstile token is single
    // use, so asking again only made people solve a second challenge.
    const onSubmit = async (e: React.FormEvent) => { e.preventDefault(); if (!pending) await submit(); };

    return { mail, otp, setOtp, pending, onSubmit };
}
