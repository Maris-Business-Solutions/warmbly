// Settings layout for the configuration tabs: titled groups of rows,
// label and description on the left, the control on the right, hairlines between.

import type { ReactNode } from "react";
import { TONE_TEXT } from "@/lib/tones";
import { cn } from "@/lib/utils";

export function SettingsGroup({
    title,
    description,
    actions,
    children,
    className,
    bodyClassName,
}: {
    title: ReactNode;
    description?: ReactNode;
    actions?: ReactNode;
    children: ReactNode;
    className?: string;
    bodyClassName?: string;
}) {
    return (
        <section className={cn("mt-10 first:mt-0", className)}>
            <div className="mb-3 flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
                <div className="min-w-0 flex-1">
                    <h2 className="text-[13px] font-semibold text-foreground">{title}</h2>
                    {description && (
                        <div className="mt-1 max-w-2xl text-[12.5px] leading-relaxed text-muted-foreground">
                            {description}
                        </div>
                    )}
                </div>
                {actions && <div className="flex shrink-0 items-center gap-1.5">{actions}</div>}
            </div>
            <div
                className={cn(
                    "divide-y divide-border overflow-hidden surface-lit rounded-xl border border-border bg-card",
                    bodyClassName,
                )}
            >
                {children}
            </div>
        </section>
    );
}

export function SettingsRow({
    label,
    htmlFor,
    description,
    error,
    children,
    className,
}: {
    label: ReactNode;
    // Only for text inputs: a label pointing at a switch would toggle it twice.
    htmlFor?: string;
    description?: ReactNode;
    error?: ReactNode;
    children?: ReactNode;
    className?: string;
}) {
    return (
        <div
            className={cn(
                "flex flex-col gap-3 px-4 py-3.5 sm:flex-row sm:items-start sm:justify-between sm:gap-8",
                className,
            )}
        >
            <div className="min-w-0 flex-1">
                {htmlFor ? (
                    <label htmlFor={htmlFor} className="text-[13px] font-medium text-foreground">
                        {label}
                    </label>
                ) : (
                    <div className="text-[13px] font-medium text-foreground">{label}</div>
                )}
                {description && (
                    <div className="mt-0.5 text-[12.5px] leading-relaxed text-muted-foreground">{description}</div>
                )}
                {error && <p className={cn("mt-1 text-xs", TONE_TEXT.danger)}>{error}</p>}
            </div>
            {children && <div className="flex shrink-0 items-center gap-2 sm:min-h-6">{children}</div>}
        </div>
    );
}

// Range hint appended to a numeric row's description.
export function RangeHint({ children }: { children: ReactNode }) {
    return <span className="text-subtle-foreground">{children}</span>;
}
