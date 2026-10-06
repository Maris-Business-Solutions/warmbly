// FlowCanvas draws a laid-out flow tree as an ordinary scrolling page: the
// flow sits centred in a column, the wheel and trackpad scroll it like any
// document, and a wide flow scrolls sideways. There is no pan, zoom or grid.
// It owns nothing about what the steps mean: the builder hands it the layout,
// a renderer for each step card, and what to do when a button is pressed.

import React from "react";
import { ClockIcon, CornerDownRightIcon, PlusIcon, XIcon } from "lucide-react";
import type { BranchTone, FlowLayout, InsertPoint, PlacedEdge, PlacedNode } from "./tree";
import type { RemoteCursor } from "@/hooks/useLiveCursors";
import type { RemoteSelection } from "@/hooks/useLiveCanvas";
import Cursor from "@/components/app/presence/Cursor";
import { cn } from "@/lib/utils";

export interface FlowCanvasProps {
    layout: FlowLayout;
    renderStep: (id: string, selected: boolean, detached: boolean) => React.ReactNode;
    stepTitle: (id: string) => string;
    selectedId: string | null;
    onSelect: (id: string | null) => void;
    // Omit to make the canvas read-only (no add buttons).
    onInsert?: (at: InsertPoint, anchor: DOMRect) => void;
    onRemoveGoto?: (from: string, port: string, target: string) => void;
    // A branch pill was pressed (edit that path).
    onPillClick?: (edge: PlacedEdge) => void;
    // A note beside a line was pressed (edit the wait it shows).
    onNoteClick?: (edge: PlacedEdge, anchor: DOMRect) => void;
    // The edge whose pill is being edited, drawn ringed.
    activeEdge?: string | null;
    // The add button whose picker is open, drawn pressed.
    activeInsert?: InsertPoint | null;
    // Steps to fade back (a test run that did not reach them).
    dimmed?: Set<string> | null;
    // Scroll this step into view whenever it changes.
    revealId?: string | null;
    // Teammates on the same flow, in flow coordinates.
    cursors?: RemoteCursor[];
    selections?: RemoteSelection[];
    // Our own pointer in flow coordinates, null when it leaves.
    onCursor?: (p: { x: number; y: number } | null) => void;
    // Fill the parent and scroll inside it; otherwise grow with the flow and
    // let the page scroll.
    fill?: boolean;
    // Floating controls over the flow (see FlowOverlay).
    children?: React.ReactNode;
}

const PAD_X = 48;
const PAD_TOP = 64;
const PAD_BOTTOM = 96;

const samePoint = (a?: InsertPoint | null, b?: InsertPoint | null) =>
    !!a && !!b && a.from === b.from && a.port === b.port && a.before === b.before;

const PILL: Record<BranchTone, string> = {
    neutral: "bg-white text-slate-500 ring-slate-200",
    yes: "bg-emerald-50 text-emerald-700 ring-emerald-200/80",
    no: "bg-white text-slate-500 ring-slate-200",
    case: "bg-violet-50 text-violet-700 ring-violet-200/80",
    error: "bg-rose-50 text-rose-700 ring-rose-200/80",
    warn: "bg-amber-50 text-amber-700 ring-amber-200/80",
};

function edgePath(e: PlacedEdge): string {
    const { sx, sy, tx, ty, busY } = e;
    if (busY === null || Math.abs(tx - sx) < 0.5) return `M${sx},${sy} L${tx},${ty}`;
    const r = Math.min(10, Math.abs(tx - sx) / 2, busY - sy);
    const dir = tx > sx ? 1 : -1;
    return [
        `M${sx},${sy}`,
        `L${sx},${busY - r}`,
        `Q${sx},${busY} ${sx + dir * r},${busY}`,
        `L${tx - dir * r},${busY}`,
        `Q${tx},${busY} ${tx},${busY + r}`,
        `L${tx},${ty}`,
    ].join(" ");
}

// Absolutely placed at a flow point, centred on it.
const at = (x: number, y: number): React.CSSProperties => ({ left: x, top: y, transform: "translate(-50%, -50%)" });

// A floating control over the flow that stays put while it scrolls.
export function FlowOverlay({ position, children }: { position: "top-left" | "top-center"; children: React.ReactNode }) {
    return (
        <div className={cn("pointer-events-none absolute top-3 z-10 [&>*]:pointer-events-auto", position === "top-left" ? "left-3" : "left-1/2 -translate-x-1/2")}>
            {children}
        </div>
    );
}

export default function FlowCanvas({
    layout,
    renderStep,
    stepTitle,
    selectedId,
    onSelect,
    onInsert,
    onRemoveGoto,
    onPillClick,
    onNoteClick,
    activeEdge,
    activeInsert,
    dimmed,
    revealId,
    cursors,
    selections,
    onCursor,
    fill,
    children,
}: FlowCanvasProps) {
    const [hoverGoto, setHoverGoto] = React.useState<string | null>(null);
    const scrollRef = React.useRef<HTMLDivElement>(null);
    const layerRef = React.useRef<HTMLDivElement>(null);

    const bounds = React.useMemo(() => {
        let minX = Infinity;
        let maxX = -Infinity;
        let maxY = 0;
        for (const n of layout.nodes) {
            minX = Math.min(minX, n.x);
            maxX = Math.max(maxX, n.x + n.w);
            maxY = Math.max(maxY, n.y + n.h);
        }
        if (!layout.nodes.length) minX = maxX = 0;
        return { ox: PAD_X - minX, oy: PAD_TOP, w: maxX - minX + PAD_X * 2, h: maxY + PAD_TOP + PAD_BOTTOM };
    }, [layout]);
    const { ox, oy } = bounds;

    // A flow wider than the page opens scrolled to its first card.
    const centredOnce = React.useRef(false);
    React.useEffect(() => {
        const el = scrollRef.current;
        const root = layout.nodes[0];
        if (centredOnce.current || !el || !root) return;
        centredOnce.current = true;
        if (el.scrollWidth > el.clientWidth) el.scrollLeft = root.x + ox + root.w / 2 - el.clientWidth / 2;
    }, [layout, ox]);

    const reveal = React.useCallback((id: string) => {
        const el = layerRef.current?.querySelector(`[data-flow-step="${CSS.escape(id)}"]`);
        el?.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
    }, []);
    React.useEffect(() => {
        if (!revealId) return;
        // After a side panel opening has narrowed the view.
        const t = window.setTimeout(() => reveal(revealId), 200);
        return () => window.clearTimeout(t);
    }, [revealId, reveal]);

    const toFlow = (e: React.PointerEvent) => {
        const r = layerRef.current!.getBoundingClientRect();
        return { x: e.clientX - r.left - ox, y: e.clientY - r.top - oy };
    };

    const stepNode = (p: PlacedNode) => {
        const selected = p.key === selectedId;
        return (
            <div
                key={p.key}
                data-flow-step={p.key}
                role="button"
                tabIndex={0}
                aria-pressed={selected}
                onClick={(e) => {
                    e.stopPropagation();
                    onSelect(p.key);
                }}
                onKeyDown={(e) => {
                    if (e.target !== e.currentTarget || (e.key !== "Enter" && e.key !== " ")) return;
                    e.preventDefault();
                    onSelect(p.key);
                }}
                className={cn(
                    "absolute rounded-xl outline-none transition-opacity duration-200 focus-visible:ring-2 focus-visible:ring-sky-300",
                    dimmed?.has(p.key) && "opacity-40",
                    hoverGoto === p.key && "ring-2 ring-sky-300 ring-offset-2",
                )}
                style={{ left: p.x + ox, top: p.y + oy, width: p.w, height: p.h }}
            >
                {renderStep(p.key, selected, !!p.detached)}
            </div>
        );
    };

    const gotoNode = (p: PlacedNode) => (
        <div key={p.key} className="group absolute" style={{ left: p.x + ox, top: p.y + oy, width: p.w, height: p.h }}>
            {onRemoveGoto && p.from !== undefined && (
                <button
                    type="button"
                    aria-label="Remove this go to"
                    title="Remove this go to"
                    onClick={(e) => {
                        e.stopPropagation();
                        setHoverGoto(null);
                        onRemoveGoto(p.from!, p.port ?? "", p.ref);
                    }}
                    className="absolute -right-2 -top-2 z-10 inline-flex size-5 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-400 opacity-100 shadow-sm transition-opacity hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600 pointer-fine:opacity-0 pointer-fine:group-hover:opacity-100"
                >
                    <XIcon className="size-3" />
                </button>
            )}
            <button
                type="button"
                onClick={(e) => {
                    e.stopPropagation();
                    onSelect(p.ref);
                    reveal(p.ref);
                }}
                onMouseEnter={() => setHoverGoto(p.ref)}
                onMouseLeave={() => setHoverGoto(null)}
                onFocus={() => setHoverGoto(p.ref)}
                onBlur={() => setHoverGoto(null)}
                title="Contacts on this path continue at that step"
                className="flex h-full w-full items-center gap-1.5 rounded-full border border-dashed border-slate-300 bg-white px-3 text-[11.5px] text-slate-500 transition-colors hover:border-sky-300 hover:text-sky-700"
            >
                <CornerDownRightIcon className="size-3.5 shrink-0" />
                <span className="shrink-0">Go to</span>
                <span className="min-w-0 truncate font-medium text-slate-700">{stepTitle(p.ref)}</span>
            </button>
        </div>
    );

    const addNode = (p: PlacedNode) => {
        const point: InsertPoint = { from: p.ref, port: p.port ?? "", before: null };
        const active = samePoint(activeInsert, point);
        return (
            <div key={p.key} className="absolute" style={{ left: p.x + ox, top: p.y + oy, width: p.w, height: p.h }}>
                {onInsert ? (
                    <button
                        type="button"
                        aria-label="Add a step here"
                        title="Add a step"
                        onClick={(e) => {
                            e.stopPropagation();
                            onInsert(point, e.currentTarget.getBoundingClientRect());
                        }}
                        className={cn(
                            "flex h-full w-full items-center justify-center rounded-full border transition-colors",
                            active ? "border-sky-400 bg-sky-50 text-sky-600" : "border-slate-200 bg-white text-slate-400 hover:border-sky-400 hover:bg-sky-50 hover:text-sky-600",
                        )}
                    >
                        <PlusIcon className="size-3" />
                    </button>
                ) : (
                    <span className="mx-auto mt-2 block size-1.5 rounded-full bg-slate-300" aria-label="End of path" />
                )}
            </div>
        );
    };

    const edgeExtras = (e: PlacedEdge) => {
        const tone = e.tone ?? "neutral";
        const pillY = e.busY !== null ? e.busY + 20 : null;
        // The free stretch of line below the pill (or the whole gap) holds the + and the wait.
        const top = pillY !== null ? pillY + 12 : e.sy;
        const midY = (top + e.ty) / 2;
        const active = samePoint(activeInsert, e.insert);
        return (
            <React.Fragment key={e.key}>
                {e.label && pillY !== null && (
                    <button
                        type="button"
                        disabled={!onPillClick}
                        title={onPillClick ? `${e.label}. Click to edit this path` : e.label}
                        onClick={(ev) => {
                            ev.stopPropagation();
                            onPillClick?.(e);
                        }}
                        className={cn(
                            "absolute block max-w-[200px] truncate rounded-full px-2 py-px text-[11px] font-medium leading-[18px] ring-1 ring-inset transition-shadow disabled:cursor-default",
                            PILL[tone],
                            onPillClick && "hover:shadow-sm",
                            activeEdge === e.key && "ring-2 ring-sky-400",
                        )}
                        style={at(e.tx + ox, pillY + oy)}
                    >
                        {e.label}
                    </button>
                )}
                {e.note && (
                    <button
                        type="button"
                        disabled={!onNoteClick}
                        title={onNoteClick ? "Change the wait" : undefined}
                        onClick={(ev) => {
                            ev.stopPropagation();
                            onNoteClick?.(e, ev.currentTarget.getBoundingClientRect());
                        }}
                        className="absolute inline-flex items-center gap-1 whitespace-nowrap rounded px-1 text-[11px] text-slate-400 transition-colors enabled:hover:bg-slate-100 enabled:hover:text-slate-700 disabled:cursor-default"
                        style={{ left: e.tx + ox + 14, top: midY + oy, transform: "translateY(-50%)" }}
                    >
                        <ClockIcon className="size-3" />
                        {e.note}
                    </button>
                )}
                {onInsert && e.insert && (
                    // A hover zone over the line: the + shows only there (always on touch).
                    <div className="group/ins absolute flex w-8 items-center justify-center" style={{ ...at(e.tx + ox, midY + oy), height: Math.max(20, e.ty - top - 8) }}>
                        <button
                            type="button"
                            aria-label="Insert a step here"
                            title="Insert a step"
                            onClick={(ev) => {
                                ev.stopPropagation();
                                onInsert(e.insert!, ev.currentTarget.getBoundingClientRect());
                            }}
                            className={cn(
                                "flex size-5 items-center justify-center rounded-full border bg-white shadow-sm transition-[opacity,color,background-color,border-color] focus-visible:opacity-100",
                                active
                                    ? "border-sky-400 bg-sky-50 text-sky-600 opacity-100"
                                    : "border-slate-200 text-slate-400 opacity-100 hover:border-sky-400 hover:bg-sky-50 hover:text-sky-600 pointer-fine:opacity-0 pointer-fine:group-hover/ins:opacity-100",
                            )}
                        >
                            <PlusIcon className="size-3" />
                        </button>
                    </div>
                )}
            </React.Fragment>
        );
    };

    // The dashed line from a hovered go-to chip to the step it jumps to.
    const gotoLinks = () => {
        if (!hoverGoto) return null;
        const to = layout.nodes.find((n) => n.kind === "step" && n.ref === hoverGoto);
        if (!to) return null;
        return layout.nodes
            .filter((n) => n.kind === "goto" && n.ref === hoverGoto)
            .map((g) => {
                const right = to.x + to.w / 2 > g.x + g.w / 2;
                const sx = (right ? g.x + g.w : g.x) + ox;
                const sy = g.y + g.h / 2 + oy;
                const tx = (right ? to.x : to.x + to.w) + ox;
                const ty = to.y + to.h / 2 + oy;
                const bend = Math.max(60, Math.abs(tx - sx) / 2);
                const d = `M${sx},${sy} C${sx + (right ? bend : -bend)},${sy} ${tx + (right ? -bend : bend)},${ty} ${tx},${ty}`;
                return <path key={g.key} d={d} fill="none" stroke="var(--wb-edge-sky)" strokeWidth={1.5} strokeDasharray="5 4" />;
            });
    };

    const stepByKey = new Map(layout.nodes.filter((n) => n.kind === "step").map((n) => [n.key, n]));

    return (
        <div className={cn("relative w-full", fill && "h-full")}>
            <div ref={scrollRef} className={cn("w-full overflow-x-auto", fill && "h-full overflow-y-auto")}>
                <div
                    ref={layerRef}
                    className="relative mx-auto"
                    style={{ width: bounds.w, height: bounds.h }}
                    onClick={() => onSelect(null)}
                    onPointerMove={onCursor ? (e) => onCursor(toFlow(e)) : undefined}
                    onPointerLeave={onCursor ? () => onCursor(null) : undefined}
                >
                    <svg className="pointer-events-none absolute left-0 top-0 overflow-visible" width={bounds.w} height={bounds.h} aria-hidden="true">
                        <g transform={`translate(${ox} ${oy})`}>
                            {layout.edges.map((e) => (
                                <path key={e.key} d={edgePath(e)} fill="none" stroke={e.tone === "error" ? "var(--wb-edge-rose)" : "var(--wb-edge-faint)"} strokeWidth={1.25} />
                            ))}
                        </g>
                        {gotoLinks()}
                    </svg>
                    {layout.edges.map(edgeExtras)}
                    {layout.nodes.map((p) => (p.kind === "step" ? stepNode(p) : p.kind === "goto" ? gotoNode(p) : addNode(p)))}
                    {(selections ?? []).map((s, si) => {
                        const boxes = s.ids.map((id) => stepByKey.get(id)).filter((n): n is PlacedNode => !!n);
                        if (!boxes.length) return null;
                        const pad = 5 + (si % 3) * 3;
                        const tag = boxes.reduce((a, b) => (b.y < a.y ? b : a));
                        return boxes.map((n) => (
                            <div
                                key={`${s.userId}:${n.key}`}
                                className="pointer-events-none absolute rounded-[14px] border-2"
                                style={{ left: n.x + ox - pad, top: n.y + oy - pad, width: n.w + pad * 2, height: n.h + pad * 2, borderColor: s.color }}
                            >
                                {n.key === tag.key && (
                                    <span className="absolute -top-[19px] left-0 max-w-[160px] truncate rounded px-1.5 py-px text-[10px] font-medium leading-4 text-white" style={{ backgroundColor: s.color }}>
                                        {s.name ?? "Teammate"}
                                    </span>
                                )}
                            </div>
                        ));
                    })}
                    {cursors?.length ? (
                        <div className="pointer-events-none absolute inset-0 z-20 overflow-hidden">
                            {cursors.map((c) => (
                                <Cursor key={c.userId} color={c.color} name={c.name} avatar={c.avatar} chat={c.chat} left={c.x + ox} top={c.y + oy} />
                            ))}
                        </div>
                    ) : null}
                </div>
            </div>
            {children}
        </div>
    );
}
