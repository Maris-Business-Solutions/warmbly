// Serious, server-driven data table for the admin browsers.
//
//   - sortable column headers (server-side via onSortChange)
//   - cursor pagination (prev/next), driven by the parent's useCursorPager
//   - column show/hide + density, persisted per `storageKey`
//   - CSV export of the current page
//   - loading skeleton, ErrorState (full error + retry), and empty state
//
// The parent owns data fetching, filters, sort state, and the pager; this
// component is presentation + the table-level controls.

import { useEffect, useMemo, useRef, useState } from "react";
import {
    ArrowDown,
    ArrowUp,
    ChevronLeft,
    ChevronRight,
    ChevronsUpDown,
    Columns3,
    Download,
    Rows3,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/kit";
import { ErrorState } from "@/components/ErrorState";
import { exportCsv } from "@/lib/exportCsv";
import { cn } from "@/lib/utils";

export interface Column<T> {
    id: string;
    header: string;
    sortable?: boolean;
    sortKey?: string;
    align?: "left" | "right" | "center";
    cell: (row: T) => React.ReactNode;
    csv?: (row: T) => string | number;
    defaultHidden?: boolean;
    className?: string;
}

export interface Pager {
    canPrev: boolean;
    canNext: boolean;
    onPrev: () => void;
    onNext: () => void;
    page: number;
    shown: number;
    total?: number | null;
}

interface Props<T> {
    columns: Column<T>[];
    rows: T[];
    getRowId: (row: T) => string;
    loading?: boolean;
    error?: unknown;
    onRetry?: () => void;
    onRowClick?: (row: T) => void;
    // Highlights one row, e.g. the item open in a side pane.
    selectedRowId?: string | null;
    sort?: { by: string; desc: boolean };
    onSortChange?: (s: { by: string; desc: boolean }) => void;
    pager?: Pager;
    storageKey?: string;
    emptyTitle?: string;
    emptyHint?: string;
    csvName?: string;
    errorTitle?: string;
    /** Noun for the result count, e.g. "organizations". */
    noun?: string;
}

function load(key: string | undefined, suffix: string): string | null {
    if (!key) return null;
    try {
        return localStorage.getItem(`${key}:${suffix}`);
    } catch {
        return null;
    }
}
function save(key: string | undefined, suffix: string, value: string) {
    if (!key) return;
    try {
        localStorage.setItem(`${key}:${suffix}`, value);
    } catch {
        /* ignore */
    }
}

export function DataTable<T>({
    columns,
    rows,
    getRowId,
    loading,
    error,
    onRetry,
    onRowClick,
    selectedRowId,
    sort,
    onSortChange,
    pager,
    storageKey,
    emptyTitle = "Nothing here",
    emptyHint = "No records match these filters.",
    csvName = "export",
    errorTitle = "Failed to load",
    noun = "results",
}: Props<T>) {
    const [hidden, setHidden] = useState<Set<string>>(() => {
        const stored = load(storageKey, "cols");
        if (stored) return new Set(stored.split(",").filter(Boolean));
        return new Set(columns.filter((c) => c.defaultHidden).map((c) => c.id));
    });
    const [compact, setCompact] = useState(() => load(storageKey, "density") === "compact");
    const [colMenu, setColMenu] = useState(false);
    const menuRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (!colMenu) return;
        const onDown = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) setColMenu(false);
        };
        document.addEventListener("mousedown", onDown);
        return () => document.removeEventListener("mousedown", onDown);
    }, [colMenu]);

    const visible = useMemo(() => columns.filter((c) => !hidden.has(c.id)), [columns, hidden]);

    function toggleCol(id: string) {
        setHidden((prev) => {
            const next = new Set(prev);
            if (next.has(id)) next.delete(id);
            else next.add(id);
            save(storageKey, "cols", [...next].join(","));
            return next;
        });
    }

    function headerClick(c: Column<T>) {
        if (!c.sortable || !onSortChange) return;
        const key = c.sortKey ?? c.id;
        if (sort?.by === key) onSortChange({ by: key, desc: !sort.desc });
        else onSortChange({ by: key, desc: true });
    }

    function doExport() {
        const cols = visible.filter((c) => c.csv);
        exportCsv(
            csvName,
            cols.map((c) => c.header),
            rows.map((r) => cols.map((c) => c.csv!(r))),
        );
    }

    const rowH = compact ? "h-8" : "h-10";
    const alignCls = (a?: string) => (a === "right" ? "text-right" : a === "center" ? "text-center" : "text-left");

    const count =
        pager?.total != null
            ? `${pager.total.toLocaleString()} ${noun}`
            : `${rows.length} ${rows.length === 1 ? noun.replace(/s$/, "") : noun}`;

    const iconBtn = "grid size-7 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:opacity-40 disabled:hover:bg-transparent";

    return (
        <div>
            {/* Result count + tidy icon toolbar */}
            <div className="mb-2 flex items-center justify-between gap-3">
                <span className="text-xs tabular-nums text-muted-foreground">
                    {loading ? "Loading…" : count}
                </span>
                <div className="flex items-center gap-0.5">
                    <div className="relative" ref={menuRef}>
                        <button type="button" className={iconBtn} title="Columns" onClick={() => setColMenu((v) => !v)}>
                            <Columns3 className="size-3.5" />
                        </button>
                        {colMenu && (
                            <div className="absolute right-0 z-30 mt-1 w-52 rounded-lg bg-popover p-1 shadow-popover">
                                <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">
                                    Display properties
                                </div>
                                {columns.map((c) => (
                                    <label
                                        key={c.id}
                                        className="flex h-8 cursor-pointer items-center gap-2 rounded-[5px] px-2 text-[13px] hover:bg-accent"
                                    >
                                        <Checkbox
                                            checked={!hidden.has(c.id)}
                                            onCheckedChange={() => toggleCol(c.id)}
                                        />
                                        {c.header}
                                    </label>
                                ))}
                            </div>
                        )}
                    </div>
                    <button
                        type="button"
                        className={iconBtn}
                        title={compact ? "Comfortable rows" : "Compact rows"}
                        onClick={() => {
                            setCompact((v) => {
                                save(storageKey, "density", !v ? "compact" : "comfortable");
                                return !v;
                            });
                        }}
                    >
                        <Rows3 className="size-3.5" />
                    </button>
                    <button type="button" className={iconBtn} title="Export CSV" onClick={doExport} disabled={rows.length === 0}>
                        <Download className="size-3.5" />
                    </button>
                </div>
            </div>

            {error ? (
                <ErrorState error={error} title={errorTitle} onRetry={onRetry} />
            ) : (
                <div className="surface-lit overflow-hidden rounded-xl border border-border bg-card">
                    <div className="overflow-x-auto">
                        <table className="w-full border-collapse text-[13px]">
                            <thead>
                                <tr className="border-b border-border">
                                    {visible.map((c) => {
                                        const key = c.sortKey ?? c.id;
                                        const active = sort?.by === key;
                                        return (
                                            <th
                                                key={c.id}
                                                className={cn(
                                                    "h-9 whitespace-nowrap px-3 text-xs font-medium text-muted-foreground first:pl-4 last:pr-4",
                                                    alignCls(c.align),
                                                    c.sortable && "cursor-pointer select-none hover:text-foreground",
                                                    c.className,
                                                )}
                                                onClick={() => headerClick(c)}
                                            >
                                                <span className={cn("inline-flex items-center gap-1", c.align === "right" && "flex-row-reverse")}>
                                                    {c.header}
                                                    {c.sortable &&
                                                        (active ? (
                                                            sort!.desc ? (
                                                                <ArrowDown className="size-3 text-foreground" />
                                                            ) : (
                                                                <ArrowUp className="size-3 text-foreground" />
                                                            )
                                                        ) : (
                                                            <ChevronsUpDown className="size-3 opacity-30" />
                                                        ))}
                                                </span>
                                            </th>
                                        );
                                    })}
                                </tr>
                            </thead>
                            <tbody>
                                {loading &&
                                    Array.from({ length: 8 }).map((_, i) => (
                                        <tr key={i} className={cn("border-b border-border/70 last:border-0", rowH)}>
                                            {visible.map((c) => (
                                                <td key={c.id} className="px-3 first:pl-4 last:pr-4">
                                                    <Skeleton className="h-3.5 w-2/3" />
                                                </td>
                                            ))}
                                        </tr>
                                    ))}

                                {!loading &&
                                    rows.map((row) => (
                                        <tr
                                            key={getRowId(row)}
                                            onClick={onRowClick ? () => onRowClick(row) : undefined}
                                            className={cn(
                                                "group border-b border-border/70 last:border-0 transition-colors hover:bg-accent/50",
                                                rowH,
                                                onRowClick && "cursor-pointer",
                                                selectedRowId != null &&
                                                    getRowId(row) === selectedRowId &&
                                                    "bg-[var(--admin-accent-weak)] hover:bg-[var(--admin-accent-weak)]",
                                            )}
                                        >
                                            {visible.map((c) => (
                                                <td key={c.id} className={cn("px-3 align-middle first:pl-4 last:pr-4", alignCls(c.align), c.className)}>
                                                    {c.cell(row)}
                                                </td>
                                            ))}
                                        </tr>
                                    ))}

                                {!loading && rows.length === 0 && (
                                    <tr>
                                        <td colSpan={visible.length}>
                                            <EmptyState title={emptyTitle} hint={emptyHint} />
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            {pager && !error && (
                <div className="mt-2.5 flex items-center justify-end gap-1 text-xs text-muted-foreground">
                    <span className="mr-1.5 tabular-nums">Page {pager.page}</span>
                    <Button variant="outline" size="icon-sm" aria-label="Previous page" disabled={!pager.canPrev || loading} onClick={pager.onPrev}>
                        <ChevronLeft />
                    </Button>
                    <Button variant="outline" size="icon-sm" aria-label="Next page" disabled={!pager.canNext || loading} onClick={pager.onNext}>
                        <ChevronRight />
                    </Button>
                </div>
            )}
        </div>
    );
}
