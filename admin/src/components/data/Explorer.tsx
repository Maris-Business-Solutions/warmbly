// Two-pane data-explorer layout: a quiet facet rail on the left and the
// results beside it. The rail is flat, like a display-options panel: a
// header with the active-filter count and reset, then grouped facets.

import { Search, SlidersHorizontal, X } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import type { DateRange } from "@/lib/dateRange";
import { cn } from "@/lib/utils";

export function Explorer({
    filters,
    children,
    activeCount = 0,
    onReset,
}: {
    filters: React.ReactNode;
    children: React.ReactNode;
    activeCount?: number;
    onReset?: () => void;
}) {
    return (
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:gap-5">
            <aside className="lg:sticky lg:top-16 lg:w-60 lg:shrink-0">
                <div className="overflow-hidden rounded-lg border border-border bg-card lg:bg-transparent lg:border-0 lg:rounded-none">
                    <div className="flex h-9 items-center justify-between border-b border-border px-3 lg:px-0.5">
                        <div className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                            <SlidersHorizontal className="size-3.5 text-subtle-foreground" />
                            Filters
                            {activeCount > 0 && (
                                <span className="rounded-[4px] bg-[var(--admin-accent-soft)] px-1 text-[11px] font-medium leading-4 tabular-nums text-[var(--admin-accent-strong)]">
                                    {activeCount}
                                </span>
                            )}
                        </div>
                        {onReset && activeCount > 0 && (
                            <button
                                type="button"
                                onClick={onReset}
                                className="inline-flex h-6 items-center gap-1 rounded-md px-1.5 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                            >
                                <X className="size-3" />
                                Reset
                            </button>
                        )}
                    </div>
                    <div className="divide-y divide-border/70">{filters}</div>
                </div>
            </aside>
            <div className="min-w-0 flex-1">{children}</div>
        </div>
    );
}

export function FilterGroup({ label, children }: { label?: string; children: React.ReactNode }) {
    return (
        <div className="px-3 py-3 lg:px-0.5">
            {label && (
                <div className="mb-1.5 text-xs font-medium text-muted-foreground">
                    {label}
                </div>
            )}
            <div className="space-y-1.5">{children}</div>
        </div>
    );
}

export function SearchFilter({
    value,
    onChange,
    placeholder = "Search…",
}: {
    value: string;
    onChange: (v: string) => void;
    placeholder?: string;
}) {
    return (
        <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
                value={value}
                onChange={(e) => onChange(e.target.value)}
                placeholder={placeholder}
                className="h-8 pl-8"
            />
        </div>
    );
}

export function SegmentedFilter<T extends string>({
    value,
    onChange,
    options,
}: {
    value: T;
    onChange: (v: T) => void;
    options: { value: T; label: string }[];
}) {
    return (
        <div className="grid grid-cols-3 gap-0.5 rounded-md border border-border bg-muted/50 p-0.5 text-[12px]">
            {options.map((o) => (
                <button
                    key={o.value}
                    type="button"
                    onClick={() => onChange(o.value)}
                    className={cn(
                        "h-6 min-w-0 truncate whitespace-nowrap rounded-[5px] px-1.5 font-medium transition-colors",
                        value === o.value
                            ? "bg-card text-foreground shadow-[0_1px_2px_rgb(0_0_0/0.08)] dark:bg-accent"
                            : "text-muted-foreground hover:text-foreground",
                    )}
                >
                    {o.label}
                </button>
            ))}
        </div>
    );
}

export function SelectFilter<T extends string>({
    value,
    onChange,
    options,
    placeholder = "Any",
}: {
    value: T;
    onChange: (v: T) => void;
    options: { value: T; label: string }[];
    placeholder?: string;
}) {
    return (
        <Select value={value || undefined} onValueChange={(v) => onChange(v as T)}>
            <SelectTrigger className="w-full">
                <SelectValue placeholder={placeholder} />
            </SelectTrigger>
            <SelectContent>
                {options.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                        {o.label}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
}

export function ToggleFilter({
    checked,
    onChange,
    label,
}: {
    checked: boolean;
    onChange: (v: boolean) => void;
    label: string;
}) {
    return (
        <label className="flex h-7 cursor-pointer items-center gap-2 rounded-md text-[13px] text-foreground">
            <Checkbox checked={checked} onCheckedChange={(v) => onChange(v === true)} />
            {label}
        </label>
    );
}

// Timeline / date-range facet: a preset dropdown (Any / Last 24h / 7d / 30d /
// 90d / Custom) that reveals two themed date inputs when "Custom" is chosen.
// mode="custom" drops the day-presets (for timestamps where "last N days" is
// meaningless, e.g. a trial-end or scheduled-deletion date).
export function DateRangeFilter({
    value,
    onChange,
    mode = "preset",
}: {
    value: DateRange;
    onChange: (v: DateRange) => void;
    mode?: "preset" | "custom";
}) {
    const options =
        mode === "preset"
            ? [
                  { value: "any", label: "Any time" },
                  { value: "1", label: "Last 24 hours" },
                  { value: "7", label: "Last 7 days" },
                  { value: "30", label: "Last 30 days" },
                  { value: "90", label: "Last 90 days" },
                  { value: "custom", label: "Custom range" },
              ]
            : [
                  { value: "any", label: "Any time" },
                  { value: "custom", label: "Custom range" },
              ];

    return (
        <div className="space-y-2">
            <SelectFilter
                value={value.preset || "any"}
                onChange={(p) =>
                    onChange(
                        p === "any"
                            ? { preset: "", after: "", before: "" }
                            : p === "custom"
                              ? { ...value, preset: "custom" }
                              : { preset: p, after: "", before: "" },
                    )
                }
                options={options}
                placeholder="Any time"
            />
            {value.preset === "custom" && (
                <div className="space-y-1.5">
                    <label className="block">
                        <span className="mb-1 block text-xs text-muted-foreground">From</span>
                        <Input
                            type="date"
                            value={value.after}
                            max={value.before || undefined}
                            onChange={(e) => onChange({ ...value, after: e.target.value })}
                            className="h-8"
                        />
                    </label>
                    <label className="block">
                        <span className="mb-1 block text-xs text-muted-foreground">To</span>
                        <Input
                            type="date"
                            value={value.before}
                            min={value.after || undefined}
                            onChange={(e) => onChange({ ...value, before: e.target.value })}
                            className="h-8"
                        />
                    </label>
                </div>
            )}
        </div>
    );
}

// Numeric min/max facet: two side-by-side themed inputs. Empty string => no
// bound on that side. Per CLAUDE.md we never ship a native number spinner, so
// these use inputMode=numeric on the shared text Input.
export function NumberRangeFilter({
    min,
    max,
    onMinChange,
    onMaxChange,
    minPlaceholder = "Min",
    maxPlaceholder = "Max",
}: {
    min: number | undefined;
    max: number | undefined;
    onMinChange: (v: number | undefined) => void;
    onMaxChange: (v: number | undefined) => void;
    minPlaceholder?: string;
    maxPlaceholder?: string;
}) {
    const parse = (s: string) => {
        if (s.trim() === "") return undefined;
        const n = Number(s);
        return Number.isFinite(n) ? n : undefined;
    };
    return (
        <div className="flex items-center gap-1.5">
            <Input
                inputMode="numeric"
                value={min ?? ""}
                onChange={(e) => onMinChange(parse(e.target.value))}
                placeholder={minPlaceholder}
                className="h-8 tabular-nums"
            />
            <span className="text-[11px] text-muted-foreground">–</span>
            <Input
                inputMode="numeric"
                value={max ?? ""}
                onChange={(e) => onMaxChange(parse(e.target.value))}
                placeholder={maxPlaceholder}
                className="h-8 tabular-nums"
            />
        </div>
    );
}
