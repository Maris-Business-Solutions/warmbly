// The toolbar controls of a saved view: which columns the contacts table shows
// (with drag to reorder) and how it sorts. Both write through the member's view
// preferences, so the list looks the same on their next visit and on their
// other devices.

import React from "react";
import { Reorder, useDragControls } from "framer-motion";
import { ArrowDownIcon, ArrowUpIcon, Columns3Icon, GripVerticalIcon, LockIcon, RotateCcwIcon } from "lucide-react";
import { CheckSquare } from "@/components/ui/check-square";
import type { SearchContactsSortBy } from "@/lib/api/models/app/contacts/search-contacts.types";
import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuItem,
    PopoverMenuLabel,
    PopoverMenuSeparator,
    PopoverMenuTrigger,
    SelectButton,
} from "@/components/ui/popover-menu";
import { customColumnId, type ContactColumn, type SortOption } from "./columns";

export interface ViewSortState {
    by: SearchContactsSortBy;
    reverse: boolean;
}

// Ticked columns the current list has no data for, so the chooser can say
// why a ticked column is not on screen.
const NO_EMPTY = new Set<string>();
const EmptyColumns = React.createContext<Set<string>>(NO_EMPTY);

function ColumnLabel({ col }: { col: ContactColumn }) {
    const empty = React.useContext(EmptyColumns).has(col.id);
    return (
        <>
            <span className="truncate">{col.label}</span>
            {(empty || col.custom) && (
                <span className="ml-auto shrink-0 flex items-center gap-1.5">
                    {empty && (
                        <span
                            className="text-[10px] text-slate-400 italic"
                            title="No contact in this list has a value, so the column is hidden until one does"
                        >
                            empty here
                        </span>
                    )}
                    {col.custom && <span className="text-[10px] uppercase tracking-[0.08em] text-slate-400">custom</span>}
                </span>
            )}
        </>
    );
}

// A shown column: drag by the grip (or focus it and press the arrow keys),
// click the rest of the row to hide it. A drag commits its order once, on the
// drop, not on every position crossed; a key press commits at once.
function ShownRow({
    col,
    onHide,
    onDragEnd,
    onMove,
}: {
    col: ContactColumn;
    onHide: () => void;
    onDragEnd: () => void;
    onMove: (delta: -1 | 1) => void;
}) {
    const controls = useDragControls();
    return (
        <Reorder.Item
            as="div"
            value={col}
            dragListener={false}
            dragControls={controls}
            onDragEnd={onDragEnd}
            className="relative bg-white"
        >
            <div className="mx-1 h-7 pl-1 pr-2 flex items-center gap-1.5 rounded text-[12px] text-slate-700 hover:bg-slate-100 transition-colors">
                <button
                    type="button"
                    onPointerDown={(e) => controls.start(e)}
                    onKeyDown={(e) => {
                        if (e.key !== "ArrowUp" && e.key !== "ArrowDown") return;
                        e.preventDefault();
                        onMove(e.key === "ArrowUp" ? -1 : 1);
                    }}
                    className="size-5 flex items-center justify-center rounded cursor-grab active:cursor-grabbing text-slate-300 hover:text-slate-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 focus-visible:text-slate-600 touch-none"
                    aria-label={`Move ${col.label}: drag, or press the up and down arrow keys`}
                >
                    <GripVerticalIcon className="w-3 h-3" />
                </button>
                <button type="button" onClick={onHide} className="flex-1 min-w-0 h-full flex items-center gap-2 text-left">
                    <CheckSquare checked />
                    <ColumnLabel col={col} />
                </button>
            </div>
        </Reorder.Item>
    );
}

function PlainRow({ col, checked, onClick }: { col: ContactColumn; checked: boolean; onClick: () => void }) {
    return (
        <button
            type="button"
            onClick={onClick}
            className="w-[calc(100%-8px)] mx-1 h-7 pl-1 pr-2 flex items-center gap-1.5 rounded text-[12px] text-slate-700 hover:bg-slate-100 transition-colors text-left"
        >
            <span className="size-5 shrink-0" />
            <CheckSquare checked={checked} />
            <ColumnLabel col={col} />
        </button>
    );
}

function GroupLabel({ children }: { children: React.ReactNode }) {
    return (
        <div className="px-3 pt-2 pb-1 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{children}</div>
    );
}

export function ColumnChooser({
    visible,
    available,
    customized,
    onChange,
    onReset,
    empty,
}: {
    // In display order, Name first.
    visible: ContactColumn[];
    // Built-in columns not shown, then custom fields not shown.
    available: ContactColumn[];
    customized: boolean;
    // The ids of every shown column in order, Name first.
    onChange: (ids: string[]) => void;
    onReset: () => void;
    // Shown columns hidden because the list has no data for them.
    empty?: Set<string>;
}) {
    const [open, setOpen] = React.useState(false);
    const [query, setQuery] = React.useState("");
    const q = query.trim().toLowerCase();
    const matches = (c: ContactColumn) => !q || c.label.toLowerCase().includes(q);

    const locked = visible.filter((c) => c.locked);
    const shown = visible.filter((c) => !c.locked);
    // A saved layout names every shown column including Name, so hiding the
    // last optional one saves ["name"] rather than [], which would mean the
    // default layout and bring every column back.
    const layout = (cols: ContactColumn[]) => [...locked.map((c) => c.id), ...cols.map((c) => c.id)];
    const hide = (id: string) => onChange(layout(shown.filter((x) => x.id !== id)));
    const show = (id: string) => onChange(layout([...shown, ...available.filter((c) => c.id === id)]));

    // The order being dragged lives here until the drop, so one drag is one
    // write. It follows the saved order whenever that changes from outside.
    const [order, setOrder] = React.useState(shown);
    const shownKey = shown.map((c) => c.id).join("|");
    React.useEffect(() => {
        setOrder(shown);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [shownKey]);
    const orderRef = React.useRef(order);
    orderRef.current = order;
    const commitOrder = () => {
        const next = layout(orderRef.current);
        if (next.join("|") !== layout(shown).join("|")) onChange(next);
    };
    // The keyboard path: one step up or down, saved at once.
    const moveBy = (id: string, delta: -1 | 1) => {
        const cur = orderRef.current;
        const from = cur.findIndex((c) => c.id === id);
        const to = from + delta;
        if (from < 0 || to < 0 || to >= cur.length) return;
        const next = [...cur];
        [next[from], next[to]] = [next[to], next[from]];
        setOrder(next);
        onChange(layout(next));
    };

    const builtinAvail = available.filter((c) => !c.custom && matches(c));
    const customAvail = available.filter((c) => !!c.custom && matches(c));
    const shownMatches = shown.filter(matches);
    const nothing = shownMatches.length === 0 && builtinAvail.length === 0 && customAvail.length === 0 && !locked.some(matches);

    return (
        <EmptyColumns.Provider value={empty ?? NO_EMPTY}>
        <PopoverMenu align="end" open={open} onOpenChange={(o) => { setOpen(o); if (!o) setQuery(""); }}>
            <PopoverMenuTrigger asChild>
                <SelectButton
                    icon={<Columns3Icon className="w-3.5 h-3.5" />}
                    label="Columns"
                    className={customized ? "border-sky-200 text-sky-700 hover:text-sky-800" : undefined}
                    aria-label="Choose columns"
                />
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={272} className="py-0 w-[272px]">
                <div className="px-2 py-1.5 border-b border-slate-200">
                    <input
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        placeholder="Search columns…"
                        autoFocus
                        className="w-full h-5 bg-transparent text-[12px] text-slate-900 placeholder:text-slate-400 outline-none"
                    />
                </div>
                <div className="max-h-[min(60vh,420px)] overflow-y-auto pb-1">
                    {nothing && (
                        <div className="px-3 py-4 text-[11.5px] text-slate-400 text-center">No column matches.</div>
                    )}
                    {(locked.some(matches) || shownMatches.length > 0) && (
                        <GroupLabel>{q ? "Shown" : "Shown · drag to reorder"}</GroupLabel>
                    )}
                    {locked.filter(matches).map((col) => (
                        <div
                            key={col.id}
                            className="mx-1 h-7 pl-1 pr-2 flex items-center gap-1.5 rounded text-[12px] text-slate-500"
                            title="Always shown"
                        >
                            <span className="size-5 flex items-center justify-center text-slate-300">
                                <LockIcon className="w-3 h-3" />
                            </span>
                            <CheckSquare checked />
                            <span className="truncate">{col.label}</span>
                        </div>
                    ))}
                    {q ? (
                        shownMatches.map((col) => (
                            <PlainRow key={col.id} col={col} checked onClick={() => hide(col.id)} />
                        ))
                    ) : (
                        <Reorder.Group as="div" axis="y" values={order} onReorder={setOrder}>
                            {order.map((col) => (
                                <ShownRow
                                    key={col.id}
                                    col={col}
                                    onHide={() => hide(col.id)}
                                    onDragEnd={commitOrder}
                                    onMove={(d) => moveBy(col.id, d)}
                                />
                            ))}
                        </Reorder.Group>
                    )}
                    {builtinAvail.length > 0 && (
                        <>
                            <GroupLabel>Available</GroupLabel>
                            {builtinAvail.map((col) => (
                                <PlainRow key={col.id} col={col} checked={false} onClick={() => show(col.id)} />
                            ))}
                        </>
                    )}
                    {customAvail.length > 0 && (
                        <>
                            <GroupLabel>Custom fields</GroupLabel>
                            {customAvail.map((col) => (
                                <PlainRow key={col.id} col={col} checked={false} onClick={() => show(col.id)} />
                            ))}
                        </>
                    )}
                </div>
                <div className="px-2 py-1.5 border-t border-slate-200 flex items-center justify-between gap-2">
                    <span className="text-[11px] text-slate-400 tabular-nums">
                        {visible.length} of {visible.length + available.length} shown
                    </span>
                    <button
                        type="button"
                        disabled={!customized}
                        onClick={() => { onReset(); setOpen(false); }}
                        className="h-6 px-2 rounded inline-flex items-center gap-1 text-[11.5px] text-slate-600 hover:text-slate-900 hover:bg-slate-100 disabled:opacity-40 disabled:hover:bg-transparent transition-colors"
                    >
                        <RotateCcwIcon className="w-3 h-3" />
                        Reset to default
                    </button>
                </div>
            </PopoverMenuContent>
        </PopoverMenu>
        </EmptyColumns.Provider>
    );
}

// The sort menu: every built-in sort, then the workspace's custom fields. A
// header click reaches the same state for the columns on screen; this is how
// the hidden ones, and a phone, get there.
export function SortMenu({
    sort,
    options,
    customKeys,
    onChange,
}: {
    sort: ViewSortState;
    options: SortOption[];
    customKeys: string[];
    onChange: (next: ViewSortState) => void;
}) {
    const active = [...options, ...customKeys.map((k) => ({ key: customColumnId(k) as SearchContactsSortBy, label: k }))]
        .find((o) => o.key === sort.by);
    const Dir = sort.reverse ? ArrowUpIcon : ArrowDownIcon;
    return (
        <PopoverMenu align="end">
            <PopoverMenuTrigger asChild>
                <SelectButton
                    icon={<Dir className="w-3.5 h-3.5" />}
                    label={active ? active.label : "Sort"}
                    aria-label="Sort"
                />
            </PopoverMenuTrigger>
            <PopoverMenuContent className="max-h-[min(60vh,420px)] overflow-y-auto">
                <PopoverMenuLabel>Sort by</PopoverMenuLabel>
                {options.map((o) => (
                    <PopoverMenuItem
                        key={o.key}
                        selected={sort.by === o.key}
                        onSelect={() => onChange({ by: o.key, reverse: sort.by === o.key ? sort.reverse : o.asc })}
                    >
                        {o.label}
                    </PopoverMenuItem>
                ))}
                {customKeys.length > 0 && (
                    <>
                        <PopoverMenuSeparator />
                        <PopoverMenuLabel>Custom fields</PopoverMenuLabel>
                        {customKeys.map((k) => {
                            const key = customColumnId(k) as SearchContactsSortBy;
                            return (
                                <PopoverMenuItem
                                    key={key}
                                    selected={sort.by === key}
                                    onSelect={() => onChange({ by: key, reverse: sort.by === key ? sort.reverse : true })}
                                >
                                    {k}
                                </PopoverMenuItem>
                            );
                        })}
                    </>
                )}
                <PopoverMenuSeparator />
                <PopoverMenuItem
                    selected={sort.reverse}
                    onSelect={() => onChange({ ...sort, reverse: !sort.reverse })}
                    closeOnSelect={false}
                >
                    Reverse order
                </PopoverMenuItem>
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
