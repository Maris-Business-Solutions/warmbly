// Building blocks every page composes from, so the pages read as
// one product: sections, stat tiles, property lists, status badges, empty
// states and a segmented control. Prefer these over hand-rolled markup.

import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Skeleton } from "@/components/ui/skeleton";
import { TONE, TONE_DOT, TONE_PANEL, TONE_TEXT, type Tone } from "@/lib/tones";
import { cn } from "@/lib/utils";

// ---- Section ---------------------------------------------------------------

// A titled block of a page. Title 13px semibold, optional hint and actions.
export function Section({
    title,
    description,
    actions,
    children,
    className,
}: {
    title?: ReactNode;
    description?: ReactNode;
    actions?: ReactNode;
    children: ReactNode;
    className?: string;
}) {
    return (
        <section className={cn("mt-8 first:mt-0", className)}>
            {(title || actions) && (
                <div className="mb-3 flex flex-wrap items-end justify-between gap-2">
                    <div className="min-w-0">
                        {title && <h2 className="text-[13px] font-semibold text-foreground">{title}</h2>}
                        {description && (
                            <p className="mt-0.5 max-w-3xl text-[12.5px] leading-relaxed text-muted-foreground">{description}</p>
                        )}
                    </div>
                    {actions && <div className="flex items-center gap-1.5">{actions}</div>}
                </div>
            )}
            {children}
        </section>
    );
}

// A bordered surface with an optional header row. The default container for
// grouped content inside a section.
export function Panel({
    title,
    description,
    actions,
    children,
    className,
    bodyClassName,
}: {
    title?: ReactNode;
    description?: ReactNode;
    actions?: ReactNode;
    children: ReactNode;
    className?: string;
    bodyClassName?: string;
}) {
    return (
        <div className={cn("surface-lit overflow-hidden rounded-xl border border-border bg-card", className)}>
            {(title || actions) && (
                <div className="flex min-h-11 items-center justify-between gap-3 border-b border-border px-4 py-2">
                    <div className="min-w-0">
                        {title && <div className="truncate text-[13px] font-medium text-foreground">{title}</div>}
                        {description && <div className="truncate text-xs text-muted-foreground">{description}</div>}
                    </div>
                    {actions && <div className="flex shrink-0 items-center gap-1.5">{actions}</div>}
                </div>
            )}
            <div className={cn("p-4", bodyClassName)}>{children}</div>
        </div>
    );
}

// ---- Stats -----------------------------------------------------------------

// A row of metrics joined into one bordered strip, divided by hairlines.
export function StatGrid({ children, className }: { children: ReactNode; className?: string }) {
    return (
        <div
            className={cn(
                "surface-lit grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-border bg-border md:grid-cols-4",
                className,
            )}
        >
            {children}
        </div>
    );
}

export function Stat({
    label,
    value,
    sub,
    icon: Icon,
    tone,
    loading,
    className,
}: {
    label: ReactNode;
    value: ReactNode;
    sub?: ReactNode;
    icon?: LucideIcon;
    tone?: Tone;
    loading?: boolean;
    className?: string;
}) {
    return (
        <div className={cn("bg-card px-4 py-3.5", className)}>
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                {Icon && <Icon className="size-3.5 text-subtle-foreground" />}
                {label}
            </div>
            <div
                className={cn(
                    "mt-1.5 text-2xl leading-8 font-semibold tracking-[-0.03em] tabular-nums",
                    tone ? TONE_TEXT[tone] : "text-foreground",
                )}
            >
                {loading ? <Skeleton className="my-1 h-5 w-16" /> : value}
            </div>
            {sub && <div className="mt-0.5 truncate text-xs text-muted-foreground">{sub}</div>}
        </div>
    );
}

// ---- Properties ------------------------------------------------------------

// Label / value rows for a detail page's properties column.
export function PropertyList({
    title,
    children,
    className,
}: {
    title?: ReactNode;
    children: ReactNode;
    className?: string;
}) {
    const list = <dl className={cn("divide-y divide-border", !title && className)}>{children}</dl>;
    if (!title) return list;
    return (
        <div className={className}>
            <div className="mb-1 text-xs font-medium text-muted-foreground">{title}</div>
            {list}
        </div>
    );
}

export function Property({
    label,
    children,
    className,
}: {
    label: ReactNode;
    children: ReactNode;
    className?: string;
}) {
    return (
        <div className={cn("grid grid-cols-[minmax(7rem,38%)_1fr] items-baseline gap-3 py-2 text-[13px]", className)}>
            <dt className="truncate text-muted-foreground">{label}</dt>
            <dd className="min-w-0 break-words text-foreground">{children}</dd>
        </div>
    );
}

// ---- Status ----------------------------------------------------------------

// Tinted pill for an enum state. `dot` adds a leading status dot.
export function StatusBadge({
    tone = "neutral",
    dot,
    children,
    className,
    title,
}: {
    tone?: Tone;
    dot?: boolean;
    children: ReactNode;
    className?: string;
    title?: string;
}) {
    return (
        <span
            title={title}
            className={cn(
                "inline-flex h-5 w-fit shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border px-2 text-[11.5px] font-medium",
                TONE[tone],
                className,
            )}
        >
            {dot && <span className={cn("size-1.5 rounded-full", TONE_DOT[tone])} />}
            {children}
        </span>
    );
}

// Dot + label with no pill, for dense table cells.
export function StatusDot({
    tone = "neutral",
    children,
    pulse,
    className,
}: {
    tone?: Tone;
    children?: ReactNode;
    pulse?: boolean;
    className?: string;
}) {
    return (
        <span className={cn("inline-flex items-center gap-1.5 text-[13px]", className)}>
            <span className="relative flex size-2 shrink-0">
                {pulse && <span className={cn("absolute inset-0 animate-ping rounded-full opacity-50", TONE_DOT[tone])} />}
                <span className={cn("relative size-2 rounded-full", TONE_DOT[tone])} />
            </span>
            {children}
        </span>
    );
}

// Inline notice: a bordered wash with an icon, title and body.
export function Callout({
    tone = "neutral",
    icon: Icon,
    title,
    children,
    actions,
    className,
}: {
    tone?: Tone;
    icon?: LucideIcon;
    title?: ReactNode;
    children?: ReactNode;
    actions?: ReactNode;
    className?: string;
}) {
    return (
        <div className={cn("flex items-start gap-3 rounded-lg border px-3.5 py-3", TONE_PANEL[tone], className)}>
            {Icon && <Icon className={cn("mt-0.5 size-4 shrink-0", TONE_TEXT[tone])} />}
            <div className="min-w-0 flex-1 text-[13px]">
                {title && <div className="font-medium text-foreground">{title}</div>}
                {children && <div className={cn("leading-relaxed text-muted-foreground", title && "mt-0.5")}>{children}</div>}
            </div>
            {actions && <div className="flex shrink-0 items-center gap-1.5">{actions}</div>}
        </div>
    );
}

// ---- Empty -----------------------------------------------------------------

export function EmptyState({
    icon: Icon,
    title,
    hint,
    action,
    className,
}: {
    icon?: LucideIcon;
    title: ReactNode;
    hint?: ReactNode;
    action?: ReactNode;
    className?: string;
}) {
    return (
        <div className={cn("flex flex-col items-center justify-center px-6 py-14 text-center", className)}>
            {Icon && (
                <span className="mb-3 grid size-10 place-items-center rounded-lg border border-border bg-muted/50 text-subtle-foreground">
                    <Icon className="size-5" />
                </span>
            )}
            <div className="text-[13px] font-medium text-foreground">{title}</div>
            {hint && <div className="mt-1 max-w-sm text-[12.5px] leading-relaxed text-muted-foreground">{hint}</div>}
            {action && <div className="mt-4">{action}</div>}
        </div>
    );
}

// ---- Segmented control ------------------------------------------------------

export function Segmented<T extends string | number>({
    value,
    onChange,
    options,
    ariaLabel,
    disabled,
    fullWidth,
    className,
}: {
    value: T;
    onChange: (v: T) => void;
    options: { value: T; label: ReactNode; disabled?: boolean }[];
    ariaLabel?: string;
    disabled?: boolean;
    fullWidth?: boolean;
    className?: string;
}) {
    return (
        <div
            role="radiogroup"
            aria-label={ariaLabel}
            className={cn(
                "h-7 items-center rounded-md border border-border bg-muted/50 p-0.5",
                fullWidth ? "flex w-full" : "inline-flex",
                className,
            )}
        >
            {options.map((o) => {
                const active = o.value === value;
                return (
                    <button
                        key={String(o.value)}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        disabled={disabled || o.disabled}
                        onClick={() => onChange(o.value)}
                        className={cn(
                            "h-full min-w-0 truncate rounded-[5px] px-2 text-[12px] font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40 disabled:pointer-events-none disabled:opacity-50",
                            fullWidth && "flex-1 basis-0",
                            active
                                ? "bg-card text-foreground shadow-[0_1px_2px_rgb(0_0_0/0.08)] dark:bg-accent"
                                : "text-muted-foreground hover:text-foreground",
                        )}
                    >
                        {o.label}
                    </button>
                );
            })}
        </div>
    );
}

// ---- Kbd ---------------------------------------------------------------------

export function Kbd({ children, className }: { children: ReactNode; className?: string }) {
    return (
        <kbd
            className={cn(
                "inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-[4px] border border-border bg-muted px-1 font-sans text-[11px] font-medium text-muted-foreground",
                className,
            )}
        >
            {children}
        </kbd>
    );
}
