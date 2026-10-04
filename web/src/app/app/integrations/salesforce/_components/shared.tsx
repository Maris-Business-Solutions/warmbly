// Building blocks shared by the Salesforce settings tabs.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { ChevronDownIcon, Loader2Icon, PlusIcon, XIcon } from "lucide-react";

import { CheckSquare } from "@/components/ui/check-square";
import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuItem,
    PopoverMenuTrigger,
} from "@/components/ui/popover-menu";
import useClickOutside from "@/hooks/useClickOutside";
import useFlipPlacement from "@/hooks/useFlipPlacement";
import type { SalesforcePicklistValue } from "@/lib/api/models/app/integrations/Salesforce";
import { cn } from "@/lib/utils";

export function SectionLabel({ children, className }: { children: React.ReactNode; className?: string }) {
    return (
        <div className={cn("text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium", className)}>
            {children}
        </div>
    );
}

// A titled settings block: label + optional description above a bordered card.
export function SettingsCard({
    title,
    description,
    action,
    children,
}: {
    title: string;
    description?: React.ReactNode;
    action?: React.ReactNode;
    children: React.ReactNode;
}) {
    return (
        <section className="space-y-2">
            <div className="flex items-end justify-between gap-3">
                <div className="min-w-0">
                    <SectionLabel>{title}</SectionLabel>
                    {description && (
                        <p className="text-[11.5px] text-slate-500 mt-1 leading-relaxed max-w-2xl">{description}</p>
                    )}
                </div>
                {action}
            </div>
            <div className="rounded-md border border-slate-200 bg-white divide-y divide-slate-100">{children}</div>
        </section>
    );
}

// One row inside a SettingsCard.
export function CardRow({ children, className }: { children: React.ReactNode; className?: string }) {
    return <div className={cn("px-4 py-3", className)}>{children}</div>;
}

export interface SearchOption {
    value: string;
    label: string;
    hint?: string;
    disabled?: boolean;
    disabledReason?: string;
}

// A single-select with a search header, for option lists too long to scan.
// Pass onQueryChange to search server-side instead of filtering locally.
export function SearchSelect({
    value,
    onChange,
    options,
    placeholder = "Select…",
    searchPlaceholder = "Search…",
    valueLabel,
    onQueryChange,
    loading,
    emptyText = "Nothing matches.",
    className,
    minWidth = 280,
    disabled,
    footer,
    "aria-label": ariaLabel,
}: {
    value: string;
    onChange: (value: string, option: SearchOption) => void;
    options: SearchOption[];
    placeholder?: string;
    searchPlaceholder?: string;
    // Label for a value not in the current options (a saved id, a remote search).
    valueLabel?: string;
    onQueryChange?: (q: string) => void;
    loading?: boolean;
    emptyText?: string;
    className?: string;
    minWidth?: number;
    disabled?: boolean;
    footer?: React.ReactNode;
    "aria-label"?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState("");
    const current = options.find((o) => o.value === value);
    const label = current?.label ?? (value ? valueLabel || value : "");

    const q = query.trim().toLowerCase();
    const shown = onQueryChange
        ? options
        : q
          ? options.filter((o) => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q))
          : options;

    function setQ(v: string) {
        setQuery(v);
        onQueryChange?.(v);
    }

    return (
        <PopoverMenu
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (!o) setQ("");
            }}
        >
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    disabled={disabled}
                    aria-label={ariaLabel}
                    className={cn(
                        "h-7 px-2.5 rounded-md border border-slate-200 hover:border-slate-300 bg-white text-[12px] text-slate-700 flex items-center gap-1.5 transition-colors disabled:opacity-60 disabled:cursor-not-allowed min-w-0",
                        className,
                    )}
                >
                    <span className={cn("truncate flex-1 text-left", !label && "text-slate-400")}>
                        {label || placeholder}
                    </span>
                    <ChevronDownIcon className="w-3 h-3 text-slate-400 shrink-0" />
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={minWidth} className="max-h-80">
                <div className="px-2 py-1.5 border-b border-slate-200 sticky top-0 bg-white z-[1] flex items-center gap-1.5">
                    <input
                        value={query}
                        onChange={(e) => setQ(e.target.value)}
                        placeholder={searchPlaceholder}
                        autoFocus
                        className="flex-1 min-w-0 h-5 bg-transparent text-[12px] text-slate-900 placeholder:text-slate-400 outline-none"
                    />
                    {loading && <Loader2Icon className="w-3 h-3 animate-spin text-slate-400 shrink-0" />}
                </div>
                {shown.map((o) => (
                    <PopoverMenuItem
                        key={o.value}
                        selected={o.value === value}
                        disabled={o.disabled}
                        onSelect={() => onChange(o.value, o)}
                    >
                        <span className="flex flex-col min-w-0" title={o.disabled ? o.disabledReason : undefined}>
                            <span className="truncate">{o.label}</span>
                            {(o.hint || (o.disabled && o.disabledReason)) && (
                                <span className="text-[10.5px] text-slate-400 truncate font-mono">
                                    {o.disabled && o.disabledReason ? o.disabledReason : o.hint}
                                </span>
                            )}
                        </span>
                    </PopoverMenuItem>
                ))}
                {shown.length === 0 && (
                    <div className="px-3 py-3 text-[11.5px] text-slate-400 text-center">
                        {loading ? "Loading…" : emptyText}
                    </div>
                )}
                {footer}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

// A Salesforce picklist value, with an explicit empty choice ("Don't change").
export function PicklistSelect({
    value,
    onChange,
    values,
    emptyLabel = "Don't change",
    className,
    loading,
    "aria-label": ariaLabel,
}: {
    value: string;
    onChange: (v: string) => void;
    values: SalesforcePicklistValue[];
    emptyLabel?: string;
    className?: string;
    loading?: boolean;
    "aria-label"?: string;
}) {
    const options: SearchOption[] = [
        { value: "", label: emptyLabel },
        ...values.map((v) => ({ value: v.value, label: v.label || v.value })),
    ];
    // A saved value the org no longer offers still shows, so it can be cleared.
    if (value && !values.some((v) => v.value === value)) {
        options.push({ value, label: `${value} (not in Salesforce)` });
    }
    return (
        <SearchSelect
            value={value}
            onChange={(v) => onChange(v)}
            options={options}
            placeholder={emptyLabel}
            searchPlaceholder="Search statuses…"
            loading={loading}
            className={className}
            minWidth={240}
            aria-label={ariaLabel}
        />
    );
}

// Multi-select chips with a searchable dropdown, in the contacts CategoryPicker style.
export function ChipMultiPicker({
    value,
    onChange,
    options,
    placeholder = "Click to add…",
    className,
}: {
    value: string[];
    onChange: (next: string[]) => void;
    options: SalesforcePicklistValue[];
    placeholder?: string;
    className?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState("");
    const ref = React.useRef<HTMLDivElement>(null);
    const triggerRef = React.useRef<HTMLDivElement>(null);
    useClickOutside(open, () => setOpen(false), ref);
    const placement = useFlipPlacement(triggerRef, open, 270);

    const labelOf = (v: string) => options.find((o) => o.value === v)?.label || v;
    const q = query.trim().toLowerCase();
    const filtered = q
        ? options.filter((o) => (o.label || o.value).toLowerCase().includes(q))
        : options;

    function toggle(v: string) {
        onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]);
    }

    return (
        <div ref={ref} className={cn("relative", className)}>
            <div ref={triggerRef} className="rounded-md border border-slate-200 bg-white min-h-[34px]">
                {value.length === 0 ? (
                    <button
                        type="button"
                        onClick={() => setOpen((o) => !o)}
                        className="w-full text-left px-3 py-2 text-[11.5px] text-slate-400 hover:text-slate-600"
                    >
                        {placeholder}
                    </button>
                ) : (
                    <div className="px-2 py-2 flex flex-wrap gap-1">
                        {value.map((v) => (
                            <span
                                key={v}
                                className="inline-flex items-center gap-1 h-5 pl-1.5 pr-1 rounded border border-sky-200 bg-sky-50 text-[11px] font-medium text-sky-700"
                            >
                                <span className="truncate max-w-[160px]">{labelOf(v)}</span>
                                <button
                                    type="button"
                                    onClick={() => toggle(v)}
                                    aria-label={`Remove ${labelOf(v)}`}
                                    className="opacity-70 hover:opacity-100"
                                >
                                    <XIcon className="w-2.5 h-2.5" />
                                </button>
                            </span>
                        ))}
                        <button
                            type="button"
                            onClick={() => setOpen((o) => !o)}
                            className="inline-flex items-center gap-1 h-5 px-1.5 rounded text-[11px] font-medium border border-dashed border-slate-300 text-slate-500 hover:border-slate-400 hover:text-slate-700"
                        >
                            <PlusIcon className="w-2.5 h-2.5" />
                            Add
                        </button>
                    </div>
                )}
            </div>
            <AnimatePresence>
                {open && (
                    <motion.div
                        data-floating
                        initial={{ opacity: 0, y: placement === "top" ? 4 : -4 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: placement === "top" ? 4 : -4 }}
                        transition={{ duration: 0.12 }}
                        className={cn(
                            "absolute left-0 right-0 z-30 rounded-md border border-slate-200 bg-white shadow-[0_12px_32px_-8px_rgba(15,23,42,0.18)] overflow-hidden",
                            placement === "top" ? "bottom-full mb-1" : "top-full mt-1",
                        )}
                    >
                        <div className="px-2 py-1.5 border-b border-slate-200">
                            <input
                                value={query}
                                onChange={(e) => setQuery(e.target.value)}
                                placeholder="Search…"
                                autoFocus
                                className="w-full h-5 bg-transparent text-[12px] text-slate-900 placeholder:text-slate-400 outline-none"
                            />
                        </div>
                        <div className="max-h-56 overflow-y-auto py-1">
                            {filtered.length === 0 && (
                                <div className="px-3 py-3 text-[11.5px] text-slate-400 text-center">Nothing matches.</div>
                            )}
                            {filtered.map((o) => (
                                <button
                                    key={o.value}
                                    type="button"
                                    onClick={() => toggle(o.value)}
                                    className="w-full px-2.5 h-7 flex items-center gap-2 text-[12px] text-slate-700 hover:bg-slate-100 transition-colors"
                                >
                                    <CheckSquare checked={value.includes(o.value)} />
                                    <span className="truncate">{o.label || o.value}</span>
                                </button>
                            ))}
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    );
}

// A labelled toggle row: title and help on the left, the control on the right.
export function ToggleRow({
    title,
    description,
    control,
    children,
}: {
    title: React.ReactNode;
    description?: React.ReactNode;
    control: React.ReactNode;
    children?: React.ReactNode;
}) {
    return (
        <div className="flex items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
                <div className="text-[12.5px] text-slate-900 font-medium">{title}</div>
                {description && <p className="text-[11px] text-slate-500 mt-0.5 leading-relaxed">{description}</p>}
                {children}
            </div>
            <div className="shrink-0 pt-0.5">{control}</div>
        </div>
    );
}

export function Pill({
    tone = "slate",
    children,
    className,
    title,
}: {
    tone?: "slate" | "sky" | "emerald" | "amber" | "rose";
    children: React.ReactNode;
    className?: string;
    title?: string;
}) {
    const cls = {
        slate: "bg-slate-100 text-slate-600 border-slate-200",
        sky: "bg-sky-50 text-sky-700 border-sky-100",
        emerald: "bg-emerald-50 text-emerald-700 border-emerald-100",
        amber: "bg-amber-50 text-amber-700 border-amber-200",
        rose: "bg-rose-50 text-rose-700 border-rose-100",
    }[tone];
    return (
        <span
            title={title}
            className={cn(
                "inline-flex items-center gap-1 h-5 px-1.5 rounded border text-[10px] font-medium whitespace-nowrap",
                cls,
                className,
            )}
        >
            {children}
        </span>
    );
}

export const secondaryBtn =
    "h-7 px-2.5 rounded-md border border-slate-200 bg-white hover:border-slate-300 text-slate-700 hover:text-slate-900 text-[12px] inline-flex items-center gap-1.5 transition-colors disabled:opacity-60 disabled:cursor-not-allowed";

export const primaryBtn =
    "h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60 disabled:cursor-not-allowed";
