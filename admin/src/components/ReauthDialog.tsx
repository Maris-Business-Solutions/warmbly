// Global "confirm it is you" prompt, the admin's mirror of the dashboard's
// ReauthModal. The API client dispatches `reauth-required` with resolve and
// reject when a route answers reauth_required, and retries once on resolve.

import { useCallback, useEffect, useState, type FormEvent } from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { reauth } from "@/lib/api/client/auth";
import { APIError } from "@/lib/api/client";

interface ReauthDetail {
    resolve: () => void;
    reject: () => void;
    ack?: () => void;
}

export function ReauthDialog() {
    const [pending, setPending] = useState<ReauthDetail | null>(null);
    const [password, setPassword] = useState("");
    const [code, setCode] = useState("");
    const [error, setError] = useState("");
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        const handler = (e: Event) => {
            setPassword("");
            setCode("");
            setError("");
            setBusy(false);
            const detail = (e as CustomEvent<ReauthDetail>).detail;
            detail.ack?.();
            setPending(detail);
        };
        window.addEventListener("reauth-required", handler);
        return () => window.removeEventListener("reauth-required", handler);
    }, []);

    const cancel = useCallback(() => {
        pending?.reject();
        setPending(null);
    }, [pending]);

    async function submit(e: FormEvent) {
        e.preventDefault();
        if (busy || !pending) return;
        setBusy(true);
        setError("");
        try {
            await reauth({ password: password || undefined, code: code || undefined });
            pending.resolve();
            setPending(null);
        } catch (err) {
            // One message for both factors; which one matched is not worth saying.
            setError(
                err instanceof APIError && (err.code === "reauth_no_factor" || err.code === "reauth_limited")
                    ? err.message
                    : "That did not match. Try again.",
            );
            setBusy(false);
        }
    }

    return (
        <DialogPrimitive.Root open={pending !== null} onOpenChange={(open) => !open && cancel()}>
            <DialogPrimitive.Portal>
                <DialogPrimitive.Overlay className="data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-[70] bg-black/50" />
                <DialogPrimitive.Content
                    role="alertdialog"
                    data-floating=""
                    onMouseDown={(e) => e.stopPropagation()}
                    className="bg-background data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] z-[70] w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] rounded-xl border border-border p-6 shadow-lg duration-200 outline-none sm:max-w-sm"
                >
                    <form onSubmit={submit} className="grid gap-4">
                        <div className="flex items-start gap-3">
                            <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-muted text-foreground">
                                <ShieldCheck className="size-4" />
                            </span>
                            <div className="min-w-0 flex flex-col gap-1.5">
                                <DialogPrimitive.Title className="text-base leading-snug font-semibold text-foreground">
                                    Confirm it is you
                                </DialogPrimitive.Title>
                                <DialogPrimitive.Description className="text-sm text-muted-foreground">
                                    This change needs a fresh check. Enter your password, or a code from your
                                    authenticator.
                                </DialogPrimitive.Description>
                            </div>
                        </div>
                        <div className="grid gap-2.5">
                            <Input
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                placeholder="Password"
                                autoComplete="current-password"
                                autoFocus
                            />
                            <Input
                                type="text"
                                value={code}
                                onChange={(e) => setCode(e.target.value)}
                                placeholder="Two-factor or recovery code"
                                autoComplete="one-time-code"
                                data-ph-mask=""
                            />
                        </div>
                        {error && <p className="text-sm text-[var(--admin-danger)]">{error}</p>}
                        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                            <Button type="button" variant="outline" onClick={cancel}>
                                Cancel
                            </Button>
                            <Button type="submit" disabled={busy || (!password && !code)}>
                                {busy ? "Checking…" : "Confirm"}
                            </Button>
                        </div>
                    </form>
                </DialogPrimitive.Content>
            </DialogPrimitive.Portal>
        </DialogPrimitive.Root>
    );
}
