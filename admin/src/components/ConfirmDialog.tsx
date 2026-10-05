// In-app confirmation, the admin's replacement for window.confirm.
//
//   const confirm = useConfirm();
//   if (!(await confirm({ title, description, confirmLabel, destructive }))) return;
//
// One provider (mounted in the app shell) renders a Radix Dialog with
// role="alertdialog"; Escape, the backdrop and Cancel all resolve false.

import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

export interface ConfirmOptions {
    title: string;
    description?: ReactNode;
    confirmLabel?: string;
    cancelLabel?: string;
    destructive?: boolean;
}

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

export function ConfirmProvider({ children }: { children: ReactNode }) {
    const [options, setOptions] = useState<ConfirmOptions | null>(null);
    const resolver = useRef<((ok: boolean) => void) | null>(null);

    const settle = useCallback((ok: boolean) => {
        resolver.current?.(ok);
        resolver.current = null;
        setOptions(null);
    }, []);

    const confirm = useCallback<ConfirmFn>(
        (next) => {
            // A second call while one is open answers the first with "no".
            resolver.current?.(false);
            setOptions(next);
            return new Promise<boolean>((resolve) => {
                resolver.current = resolve;
            });
        },
        [],
    );

    return (
        <ConfirmContext.Provider value={confirm}>
            {children}
            <DialogPrimitive.Root open={options !== null} onOpenChange={(open) => !open && settle(false)}>
                <DialogPrimitive.Portal>
                    <DialogPrimitive.Overlay className="data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-[60] bg-black/40 backdrop-blur-[2px] dark:bg-black/60" />
                    <DialogPrimitive.Content
                        role="alertdialog"
                        data-floating=""
                        onMouseDown={(e) => e.stopPropagation()}
                        className="bg-background dark:bg-[oklch(0.19_0.004_286)] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-[0.98] data-[state=open]:zoom-in-[0.96] data-[state=open]:slide-in-from-top-[1%] ease-[cubic-bezier(0.16,1,0.3,1)] fixed top-[50%] left-[50%] z-[60] grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-xl border-0 p-5 shadow-popover duration-150 outline-none sm:max-w-md"
                    >
                        {options && (
                            <>
                                <div className="flex items-start gap-3">
                                    {options.destructive && (
                                        <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-[color-mix(in_oklab,var(--admin-danger)_12%,transparent)] text-[var(--admin-danger)]">
                                            <AlertTriangle className="size-4" />
                                        </span>
                                    )}
                                    <div className="min-w-0 flex flex-col gap-1.5">
                                        <DialogPrimitive.Title className="text-[15px] leading-snug font-semibold text-foreground">
                                            {options.title}
                                        </DialogPrimitive.Title>
                                        <DialogPrimitive.Description className="text-[13px] leading-relaxed text-muted-foreground">
                                            {options.description ?? "This action cannot be undone."}
                                        </DialogPrimitive.Description>
                                    </div>
                                </div>
                                <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
                                    <Button variant="outline" onClick={() => settle(false)}>
                                        {options.cancelLabel ?? "Cancel"}
                                    </Button>
                                    <Button
                                        autoFocus
                                        variant={options.destructive ? "destructive" : "default"}
                                        onClick={() => settle(true)}
                                    >
                                        {options.confirmLabel ?? "Confirm"}
                                    </Button>
                                </div>
                            </>
                        )}
                    </DialogPrimitive.Content>
                </DialogPrimitive.Portal>
            </DialogPrimitive.Root>
        </ConfirmContext.Provider>
    );
}

// useConfirm returns the confirm function; it throws when the provider is
// missing so a page never falls back to a silent "no".
export function useConfirm(): ConfirmFn {
    const ctx = useContext(ConfirmContext);
    if (!ctx) throw new Error("useConfirm must be used inside ConfirmProvider");
    return ctx;
}
