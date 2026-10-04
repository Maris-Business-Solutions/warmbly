// Pickers for the HubSpot settings, in the shared picker language: a bordered
// chip box (or select-style trigger) opening a PopoverMenu with a search
// header and checkbox-square rows.

import React from "react";
import { CheckIcon, ChevronDownIcon, PlusIcon, XIcon } from "lucide-react";

import { CheckSquare } from "@/components/ui/check-square";
import { PopoverMenu, PopoverMenuContent, PopoverMenuTrigger } from "@/components/ui/popover-menu";
import { cn } from "@/lib/utils";

import type { PickerOption } from "./shared";

function useFiltered(options: PickerOption[], query: string) {
    return React.useMemo(() => {
        const q = query.trim().toLowerCase();
        if (!q) return options;
        return options.filter((o) => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q));
    }, [options, query]);
}

function SearchHeader({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
    return (
        <div className="px-2 py-1.5 border-b border-slate-200">
            <input
                value={value}
                onChange={(e) => onChange(e.target.value)}
                placeholder={placeholder}
                autoFocus
                className="w-full h-5 bg-transparent text-[16px] md:text-[12px] text-slate-900 placeholder:text-slate-400 outline-none"
            />
        </div>
    );
}

function onTriggerKey(open: boolean, setOpen: (o: boolean) => void) {
    return (e: React.KeyboardEvent) => {
        if (e.key === "Enter" || e.key === " " || e.key === "ArrowDown") {
            e.preventDefault();
            setOpen(!open);
        }
    };
}

// Multi-select with chips. `emptyLabel` says what an empty selection means
// ("All pipelines"), so empty never reads as broken.
export function MultiPicker({
    options,
    selected,
    onChange,
    placeholder = "Choose…",
    emptyLabel,
    searchPlaceholder = "Search…",
    disabled,
    ariaLabel,
}: {
    options: PickerOption[];
    selected: string[];
    onChange: (next: string[]) => void;
    placeholder?: string;
    emptyLabel?: string;
    searchPlaceholder?: string;
    disabled?: boolean;
    ariaLabel?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState("");
    const filtered = useFiltered(options, query);
    const byValue = React.useMemo(() => new Map(options.map((o) => [o.value, o])), [options]);

    React.useEffect(() => {
        if (!open) setQuery("");
    }, [open]);

    function toggle(v: string) {
        onChange(selected.includes(v) ? selected.filter((s) => s !== v) : [...selected, v]);
    }

    return (
        <PopoverMenu open={open && !disabled} onOpenChange={(o) => !disabled && setOpen(o)}>
            <PopoverMenuTrigger asChild>
                <div
                    role="button"
                    tabIndex={disabled ? -1 : 0}
                    aria-label={ariaLabel}
                    aria-disabled={disabled || undefined}
                    onKeyDown={disabled ? undefined : onTriggerKey(open, setOpen)}
                    className={cn(
                        "rounded-md border border-slate-200 bg-white min-h-7 outline-none focus-visible:border-sky-400 focus-visible:ring-2 focus-visible:ring-sky-100 transition-colors",
                        disabled ? "bg-slate-50 cursor-not-allowed" : "cursor-pointer hover:border-slate-300",
                    )}
                >
                    {selected.length === 0 ? (
                        <div className="h-7 px-2.5 flex items-center gap-1.5 text-[12px]">
                            <span className={cn("flex-1 truncate", emptyLabel ? "text-slate-700" : "text-slate-400")}>
                                {emptyLabel ?? placeholder}
                            </span>
                            <ChevronDownIcon className="w-3 h-3 text-slate-400 shrink-0" />
                        </div>
                    ) : (
                        <div className="px-1.5 py-1 flex flex-wrap items-center gap-1">
                            {selected.map((v) => (
                                <span
                                    key={v}
                                    className="inline-flex items-center gap-1 h-5 max-w-full pl-1.5 pr-1 rounded bg-sky-50 text-sky-700 text-[11px] font-medium"
                                >
                                    <span className="truncate">{byValue.get(v)?.label ?? v}</span>
                                    {!disabled && (
                                        <button
                                            type="button"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                toggle(v);
                                            }}
                                            aria-label={`Remove ${byValue.get(v)?.label ?? v}`}
                                            className="opacity-70 hover:opacity-100 p-1 -m-1 md:p-0 md:m-0 shrink-0"
                                        >
                                            <XIcon className="w-2.5 h-2.5" />
                                        </button>
                                    )}
                                </span>
                            ))}
                            {!disabled && (
                                <span className="inline-flex items-center gap-1 h-5 px-1.5 rounded text-[11px] font-medium border border-dashed border-slate-300 text-slate-500">
                                    <PlusIcon className="w-2.5 h-2.5" />
                                    Add
                                </span>
                            )}
                        </div>
                    )}
                </div>
            </PopoverMenuTrigger>
            <PopoverMenuContent matchTriggerWidth minWidth={220} className="py-0">
                <SearchHeader value={query} onChange={setQuery} placeholder={searchPlaceholder} />
                <div className="max-h-60 overflow-y-auto py-1">
                    {filtered.length === 0 && (
                        <div className="px-3 py-3 text-[11.5px] text-slate-400 text-center">
                            {options.length === 0 ? "Nothing to choose from yet." : "No matches."}
                        </div>
                    )}
                    {filtered.map((o) => {
                        const checked = selected.includes(o.value);
                        return (
                            <button
                                key={o.value}
                                type="button"
                                role="menuitem"
                                data-checked={checked || undefined}
                                onClick={() => toggle(o.value)}
                                className="w-full px-2.5 min-h-7 py-1 flex items-center gap-2 text-left text-[12px] text-slate-700 hover:bg-slate-50 hover:text-slate-900 transition-colors"
                            >
                                <CheckSquare checked={checked} tone="sky" />
                                <span className="min-w-0 flex-1">
                                    <span className="block truncate">{o.label}</span>
                                    {o.hint && <span className="block truncate text-[10.5px] text-slate-400 font-mono">{o.hint}</span>}
                                </span>
                            </button>
                        );
                    })}
                </div>
                {selected.length > 0 && (
                    <button
                        type="button"
                        onClick={() => onChange([])}
                        className="w-full px-2.5 h-7 flex items-center text-[11.5px] text-slate-500 hover:bg-slate-50 hover:text-slate-800 border-t border-slate-100 transition-colors"
                    >
                        {emptyLabel ? `Reset to ${emptyLabel.toLowerCase()}` : "Clear all"}
                    </button>
                )}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

// Searchable single select, for long lists (HubSpot properties).
export function SearchSelect({
    options,
    value,
    onChange,
    placeholder = "Choose…",
    searchPlaceholder = "Search…",
    disabled,
    ariaLabel,
    className,
}: {
    options: PickerOption[];
    value: string;
    onChange: (v: string) => void;
    placeholder?: string;
    searchPlaceholder?: string;
    disabled?: boolean;
    ariaLabel?: string;
    className?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState("");
    const filtered = useFiltered(options, query);
    const current = options.find((o) => o.value === value);

    React.useEffect(() => {
        if (!open) setQuery("");
    }, [open]);

    return (
        <PopoverMenu open={open} onOpenChange={setOpen}>
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    disabled={disabled}
                    aria-label={ariaLabel}
                    className={cn(
                        "h-7 w-full px-2.5 rounded-md border border-slate-200 hover:border-slate-300 bg-white text-[12px] text-slate-700 flex items-center gap-1.5 transition-colors outline-none focus-visible:border-sky-400 focus-visible:ring-2 focus-visible:ring-sky-100 disabled:bg-slate-50 disabled:cursor-not-allowed",
                        className,
                    )}
                >
                    <span className={cn("truncate flex-1 text-left", !current && !value && "text-slate-400")}>
                        {current?.label ?? (value || placeholder)}
                    </span>
                    <ChevronDownIcon className="w-3 h-3 text-slate-400 shrink-0" />
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent matchTriggerWidth minWidth={240} className="py-0">
                <SearchHeader value={query} onChange={setQuery} placeholder={searchPlaceholder} />
                <div className="max-h-64 overflow-y-auto py-1">
                    {filtered.length === 0 && (
                        <div className="px-3 py-3 text-[11.5px] text-slate-400 text-center">
                            {options.length === 0 ? "Nothing to choose from yet." : "No matches."}
                        </div>
                    )}
                    {filtered.map((o) => {
                        const active = o.value === value;
                        return (
                            <button
                                key={o.value}
                                type="button"
                                role="menuitem"
                                data-checked={active || undefined}
                                onClick={() => {
                                    onChange(o.value);
                                    setOpen(false);
                                }}
                                className={cn(
                                    "w-full px-2.5 min-h-7 py-1 flex items-center gap-2 text-left text-[12px] hover:bg-slate-50 transition-colors",
                                    active ? "text-slate-900 font-medium" : "text-slate-700 hover:text-slate-900",
                                )}
                            >
                                <span className="min-w-0 flex-1">
                                    <span className="block truncate">{o.label}</span>
                                    {o.hint && <span className="block truncate text-[10.5px] text-slate-400 font-mono font-normal">{o.hint}</span>}
                                </span>
                                {active && <CheckIcon className="w-3 h-3 text-sky-600 shrink-0" />}
                            </button>
                        );
                    })}
                </div>
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
