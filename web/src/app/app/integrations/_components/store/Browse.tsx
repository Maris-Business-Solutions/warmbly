// The store's browse view: search, sort and every filter over the apps it is
// given, all held in the URL so a filtered view can be shared or bookmarked.
// A view that is already one category, one type or one status passes it in
// `locked`, and that filter is hidden rather than shown as a choice.

import React from "react";
import { useSearchParams } from "react-router-dom";
import {
    ArrowDownWideNarrowIcon,
    CheckIcon,
    LayoutGridIcon,
    ListIcon,
    SlidersHorizontalIcon,
    StarIcon,
    XIcon,
} from "lucide-react";

import { CheckSquare } from "@/components/ui/check-square";
import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuItem,
    PopoverMenuLabel,
    PopoverMenuSeparator,
    PopoverMenuTrigger,
    SelectButton,
} from "@/components/ui/popover-menu";
import { cn } from "@/lib/utils";

import AppCard, { AppRow, type AppCardProps } from "./AppCard";
import {
    applyFilters,
    categoryLabel,
    DEFAULT_FILTERS,
    METHOD_LABELS,
    readFilters,
    SORT_LABELS,
    STORE_CATEGORIES,
    writeFilters,
    type MethodFilter,
    type SortKey,
    type StatusFilter,
    type StoreFilters,
    type StoreItem,
    type TypeFilter,
} from "./model";

type Locked = Partial<Pick<StoreFilters, "categories" | "type" | "status">>;

const TYPE_LABELS: Record<TypeFilter, string> = { all: "All apps", official: "Official", community: "Community" };
const STATUS_LABELS: Record<StatusFilter, string> = { all: "Any status", connected: "Connected", available: "Not connected" };

export default function Browse({
    items,
    locked = {},
    isConnected,
    cardProps,
    header,
    empty,
}: {
    items: StoreItem[];
    locked?: Locked;
    isConnected: (item: StoreItem) => boolean;
    cardProps: (item: StoreItem) => Omit<AppCardProps, "highlight">;
    header?: React.ReactNode;
    empty?: React.ReactNode;
}) {
    const [params, setParams] = useSearchParams();
    const filters = React.useMemo(() => readFilters(params), [params]);
    const lockedKey = JSON.stringify(locked);
    const effective: StoreFilters = React.useMemo(() => ({ ...filters, ...(JSON.parse(lockedKey) as Locked) }), [filters, lockedKey]);

    const update = React.useCallback(
        (patch: Partial<StoreFilters>) => {
            const next = { ...readFilters(params), ...patch };
            setParams(writeFilters(next), { replace: true });
        },
        [params, setParams],
    );

    const results = React.useMemo(() => applyFilters(items, effective, isConnected), [items, effective, isConnected]);

    // Counts per category within everything else selected, so a choice shows what it would leave.
    const categoryCounts = React.useMemo(() => {
        const base = applyFilters(items, { ...effective, categories: [] }, isConnected);
        const m: Record<string, number> = {};
        for (const it of base) m[it.category] = (m[it.category] ?? 0) + 1;
        return m;
    }, [items, effective, isConnected]);

    const presentCategories = STORE_CATEGORIES.filter((c) => items.some((it) => it.category === c));
    const hasCommunity = items.some((it) => it.kind === "community");
    const hasOfficial = items.some((it) => it.kind === "builtin");

    const chips: { label: string; clear: () => void }[] = [];
    if (!locked.categories)
        for (const c of filters.categories)
            chips.push({ label: categoryLabel(c), clear: () => update({ categories: filters.categories.filter((x) => x !== c) }) });
    if (!locked.type && filters.type !== "all") chips.push({ label: TYPE_LABELS[filters.type], clear: () => update({ type: "all" }) });
    if (!locked.status && filters.status !== "all")
        chips.push({ label: STATUS_LABELS[filters.status], clear: () => update({ status: "all" }) });
    for (const m of filters.methods)
        chips.push({ label: METHOD_LABELS[m], clear: () => update({ methods: filters.methods.filter((x) => x !== m) }) });
    if (filters.featured) chips.push({ label: "Featured", clear: () => update({ featured: false }) });
    if (filters.hideSoon) chips.push({ label: "Hide coming soon", clear: () => update({ hideSoon: false }) });

    const clearAll = () =>
        update({
            q: "",
            categories: [],
            type: "all",
            status: "all",
            methods: [],
            featured: false,
            hideSoon: false,
            sort: DEFAULT_FILTERS.sort,
        });

    const sortOptions: SortKey[] = [
        ...(filters.q.trim() ? (["relevance"] as SortKey[]) : []),
        "popular",
        ...(hasCommunity ? (["installs", "newest"] as SortKey[]) : []),
        "name",
        "name_desc",
    ];

    const toggle = <T,>(list: T[], v: T) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);

    return (
        <div className="flex flex-col gap-4">
            {header}

            <div className="flex flex-col gap-2">
                <div className="flex items-center gap-1.5 flex-wrap">
                    {!locked.categories && presentCategories.length > 1 && (
                        <PopoverMenu>
                            <PopoverMenuTrigger asChild>
                                <SelectButton
                                   
                                    label={
                                        filters.categories.length === 0
                                            ? "Category"
                                            : filters.categories.length === 1
                                              ? categoryLabel(filters.categories[0])
                                              : `${filters.categories.length} categories`
                                    }
                                />
                            </PopoverMenuTrigger>
                            <PopoverMenuContent minWidth={220}>
                                <PopoverMenuLabel>Category</PopoverMenuLabel>
                                {presentCategories.map((c) => (
                                    <PopoverMenuItem
                                        key={c}
                                        closeOnSelect={false}
                                        onSelect={() => update({ categories: toggle(filters.categories, c) })}
                                        trailing={
                                            <span className="flex items-center gap-2">
                                                <span className="text-[11px] text-slate-400 tabular-nums">{categoryCounts[c] ?? 0}</span>
                                                <CheckSquare checked={filters.categories.includes(c)} tone="sky" />
                                            </span>
                                        }
                                    >
                                        {categoryLabel(c)}
                                    </PopoverMenuItem>
                                ))}
                                {filters.categories.length > 0 && (
                                    <>
                                        <PopoverMenuSeparator />
                                        <PopoverMenuItem onSelect={() => update({ categories: [] })}>Clear categories</PopoverMenuItem>
                                    </>
                                )}
                            </PopoverMenuContent>
                        </PopoverMenu>
                    )}

                    {!locked.type && hasCommunity && hasOfficial && (
                        <SingleSelect
                            label="Made by"
                            value={filters.type}
                            options={TYPE_LABELS}
                            onChange={(type) => update({ type })}
                        />
                    )}

                    {!locked.status && (
                        <SingleSelect
                            label="Status"
                            value={filters.status}
                            options={STATUS_LABELS}
                            onChange={(status) => update({ status })}
                        />
                    )}

                    <PopoverMenu>
                        <PopoverMenuTrigger asChild>
                            <SelectButton
                               
                                icon={<SlidersHorizontalIcon className="w-3.5 h-3.5" />}
                                label={
                                    filters.methods.length + Number(filters.featured) + Number(filters.hideSoon) > 0
                                        ? `More filters · ${filters.methods.length + Number(filters.featured) + Number(filters.hideSoon)}`
                                        : "More filters"
                                }
                            />
                        </PopoverMenuTrigger>
                        <PopoverMenuContent minWidth={240}>
                            <PopoverMenuLabel>Connects with</PopoverMenuLabel>
                            {(Object.keys(METHOD_LABELS) as MethodFilter[]).map((m) => (
                                <PopoverMenuItem
                                    key={m}
                                    closeOnSelect={false}
                                    onSelect={() => update({ methods: toggle(filters.methods, m) })}
                                    trailing={<CheckSquare checked={filters.methods.includes(m)} tone="sky" />}
                                >
                                    {METHOD_LABELS[m]}
                                </PopoverMenuItem>
                            ))}
                            <PopoverMenuSeparator />
                            <PopoverMenuLabel>Show</PopoverMenuLabel>
                            {hasCommunity && (
                                <PopoverMenuItem
                                    closeOnSelect={false}
                                    icon={<StarIcon className="w-3.5 h-3.5" />}
                                    onSelect={() => update({ featured: !filters.featured })}
                                    trailing={<CheckSquare checked={filters.featured} tone="sky" />}
                                >
                                    Featured only
                                </PopoverMenuItem>
                            )}
                            <PopoverMenuItem
                                closeOnSelect={false}
                                onSelect={() => update({ hideSoon: !filters.hideSoon })}
                                trailing={<CheckSquare checked={filters.hideSoon} tone="sky" />}
                            >
                                Hide coming soon
                            </PopoverMenuItem>
                        </PopoverMenuContent>
                    </PopoverMenu>

                    <div className="ml-auto flex items-center gap-2">
                        <PopoverMenu align="end">
                            <PopoverMenuTrigger asChild>
                                <SelectButton
                                   
                                    icon={<ArrowDownWideNarrowIcon className="w-3.5 h-3.5" />}
                                    label={SORT_LABELS[filters.sort]}
                                />
                            </PopoverMenuTrigger>
                            <PopoverMenuContent minWidth={190}>
                                <PopoverMenuLabel>Sort by</PopoverMenuLabel>
                                {sortOptions.map((k) => (
                                    <PopoverMenuItem
                                        key={k}
                                        selected={filters.sort === k}
                                        onSelect={() => update({ sort: k })}
                                        trailing={filters.sort === k ? <CheckIcon className="w-3.5 h-3.5 text-sky-600" /> : null}
                                    >
                                        {SORT_LABELS[k]}
                                    </PopoverMenuItem>
                                ))}
                            </PopoverMenuContent>
                        </PopoverMenu>

                        <div className="h-7 p-0.5 rounded-md border border-slate-200 bg-white inline-flex" role="group" aria-label="View">
                            {(["grid", "list"] as const).map((v) => {
                                const Icon = v === "grid" ? LayoutGridIcon : ListIcon;
                                return (
                                    <button
                                        key={v}
                                        type="button"
                                        aria-label={v === "grid" ? "Grid view" : "List view"}
                                        aria-pressed={filters.view === v}
                                        onClick={() => update({ view: v })}
                                        className={cn(
                                            "w-6 rounded inline-flex items-center justify-center transition-colors",
                                            filters.view === v ? "bg-slate-100 text-slate-900" : "text-slate-400 hover:text-slate-700",
                                        )}
                                    >
                                        <Icon className="w-3.5 h-3.5" />
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                </div>

                <div className="flex items-center gap-1.5 flex-wrap min-h-6">
                    <span className="text-[12px] text-slate-500 mr-1 tabular-nums">
                        {results.length} {results.length === 1 ? "app" : "apps"}
                        {filters.q.trim() && (
                            <>
                                {" "}
                                for <span className="font-medium text-slate-800">“{filters.q.trim()}”</span>
                            </>
                        )}
                    </span>
                    {chips.map((c) => (
                        <button
                            key={c.label}
                            type="button"
                            onClick={c.clear}
                            className="h-6 pl-2 pr-1 rounded-md bg-slate-100 text-slate-700 text-[12px] inline-flex items-center gap-1 hover:bg-slate-200/70 transition-colors"
                        >
                            {c.label}
                            <XIcon className="w-3 h-3 text-slate-400" />
                        </button>
                    ))}
                    {(chips.length > 0 || filters.q.trim()) && (
                        <button type="button" onClick={clearAll} className="text-[12px] font-medium text-slate-500 hover:text-slate-900">
                            Clear all
                        </button>
                    )}
                </div>
            </div>

            {results.length === 0 ? (
                (chips.length === 0 && !filters.q.trim() && empty) || (
                    <div className="rounded-lg border border-dashed border-slate-200 px-6 py-14 text-center">
                        <p className="text-[13px] font-medium text-slate-800">No apps match</p>
                        <p className="mt-1 text-[12.5px] text-slate-500">
                            Try fewer words or filters. Tools we don’t list can still connect through Zapier, Make or n8n.
                        </p>
                        <button
                            type="button"
                            onClick={clearAll}
                            className="mt-4 h-7 px-3 rounded-md border border-slate-200 text-[12px] font-medium text-slate-700 hover:border-slate-300"
                        >
                            Clear search and filters
                        </button>
                    </div>
                )
            ) : filters.view === "list" ? (
                <div className="rounded-lg border border-slate-200 bg-white divide-y divide-slate-200/70 overflow-hidden">
                    {results.map((it) => (
                        <AppRow key={it.key} {...cardProps(it)} highlight={filters.q} />
                    ))}
                </div>
            ) : (
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                    {results.map((it) => (
                        <AppCard key={it.key} {...cardProps(it)} highlight={filters.q} />
                    ))}
                </div>
            )}
        </div>
    );
}

function SingleSelect<T extends string>({
    label,
    value,
    options,
    onChange,
}: {
    label: string;
    value: T;
    options: Record<T, string>;
    onChange: (v: T) => void;
}) {
    const keys = Object.keys(options) as T[];
    return (
        <PopoverMenu>
            <PopoverMenuTrigger asChild>
                <SelectButton label={value === keys[0] ? label : options[value]} />
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={180}>
                <PopoverMenuLabel>{label}</PopoverMenuLabel>
                {keys.map((k) => (
                    <PopoverMenuItem
                        key={k}
                        selected={value === k}
                        onSelect={() => onChange(k)}
                        trailing={value === k ? <CheckIcon className="w-3.5 h-3.5 text-sky-600" /> : null}
                    >
                        {options[k]}
                    </PopoverMenuItem>
                ))}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
