import type React from "react";
import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSearchParams } from "@/hooks/useSearchParams";
import { useQueryClient } from "@tanstack/react-query";
import useLoginConfirm from "@/lib/api/hooks/auth/useLoginConfirm";
import { saveTokens } from "@/lib/auth";
import getUser from "@/lib/api/client/auth/getUser";
import toast from "react-hot-toast/headless";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export function useLoginConfirmForm() {
    // Query params, not path params: the login page navigates to
    // /auth/login/confirm?session=...&to=..., and the route declares no
    // path segments, so useParams() always returned empty and every
    // confirmation posted a blank session.
    const [params] = useSearchParams();
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const loginConfirm = useLoginConfirm();

    const mail = params.get("to") ?? "";
    const session = params.get("session") ?? "";
    const [otp, setOtp] = useState<string[]>(Array(6).fill(""));
    const [pending, setPending] = useState(false);

    const submit = async () => {
        setPending(true);
        try {
            const r = await toast.promise(
                loginConfirm.mutateAsync({ session, code: otp.map(v => v || "0").join("") }),
                { loading: "Loading...", success: "Successfully authorized.", error: (err: AppError) => buildError(err) }
            );
            saveTokens(Object.fromEntries(Object.entries(r).map(([k, v]) => [k, String(v)])));
            // Drop any logged-out cache, then prime the profile with the NEW token
            // BEFORE navigating into the gated shell. Navigating immediately raced
            // UserProvider's mount (useUser is refetchOnMount:false) and left the
            // loader spinning until a manual reload / window refocus.
            queryClient.clear();
            try {
                await queryClient.fetchQuery({ queryKey: ["auth", "me"], queryFn: getUser });
            } catch {
                // UserProvider re-attempts and redirects to login on a real failure.
            }
            navigate({ to: "/app/emails" });
        } finally { setPending(false); }
    };

    // No captcha here: the start step already spent one, and the signed,
    // single-use session it issued is the proof. A Turnstile token is single
    // use, so asking again only made people solve a second challenge.
    const onSubmit = async (e: React.FormEvent) => { e.preventDefault(); if (!pending) await submit(); };

    return { mail, otp, setOtp, pending, onSubmit };
}
