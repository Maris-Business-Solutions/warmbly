// FlowCanvas renders a laid-out flow tree on a pannable, zoomable surface. It
// owns nothing about what the steps mean: the builder hands it the layout, a
// renderer for each step card, and what to do when an add button is pressed.
// Cards are not draggable and lines are never drawn by hand; the structure
// decides where everything goes.

import React from "react";
import {
    ReactFlow,
    Background,
    BaseEdge,
    EdgeLabelRenderer,
    Handle,
    Panel,
    Position,
    ViewportPortal,
    useReactFlow,
    useStore,
    useStoreApi,
    useViewport,
    type Edge,
    type EdgeProps,
    type Node,
    type NodeProps,
    type ReactFlowInstance,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { CornerDownRightIcon, MaximizeIcon, MinusIcon, PlusIcon, XIcon } from "lucide-react";
import type { BranchTone, FlowLayout, InsertPoint, PlacedEdge, PlacedNode } from "./tree";
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
    // The add button whose picker is open, drawn pressed.
    activeInsert?: InsertPoint | null;
    // Steps to fade back (a test run that did not reach them).
    dimmed?: Set<string> | null;
    // Pan so this step is on screen whenever it changes.
    revealId?: string | null;
    onInit?: (inst: ReactFlowInstance) => void;
    onSelectionIds?: (ids: string[]) => void;
    children?: React.ReactNode;
}

interface CanvasCtx {
    renderStep: FlowCanvasProps["renderStep"];
    stepTitle: FlowCanvasProps["stepTitle"];
    onSelect: FlowCanvasProps["onSelect"];
    onInsert?: FlowCanvasProps["onInsert"];
    onRemoveGoto?: FlowCanvasProps["onRemoveGoto"];
    activeInsert?: InsertPoint | null;
    dimmed?: Set<string> | null;
    selectedId: string | null;
    hoverGoto: string | null;
    setHoverGoto: (key: string | null) => void;
    reveal: (id: string) => void;
}

const Ctx = React.createContext<CanvasCtx | null>(null);
const useCanvas = () => React.useContext(Ctx)!;

const samePoint = (a?: InsertPoint | null, b?: InsertPoint | null) =>
    !!a && !!b && a.from === b.from && a.port === b.port && a.before === b.before;

// Invisible anchors: React Flow only draws an edge between two handles.
function Anchors() {
    return (
        <>
            <Handle type="target" position={Position.Top} isConnectable={false} className="!pointer-events-none !opacity-0" />
            <Handle type="source" position={Position.Bottom} isConnectable={false} className="!pointer-events-none !opacity-0" />
        </>
    );
}

function StepNode({ id, data, selected }: NodeProps) {
    const c = useCanvas();
    const d = data as { placed: PlacedNode };
    const highlighted = c.hoverGoto !== null && c.hoverGoto === id;
    return (
        <div
            className={cn(
                "h-full w-full rounded-xl transition-opacity duration-200",
                c.dimmed?.has(id) && "opacity-40",
                highlighted && "ring-2 ring-sky-300 ring-offset-2 ring-offset-transparent",
            )}
        >
            <Anchors />
            {c.renderStep(id, !!selected, !!d.placed.detached)}
        </div>
    );
}

function GotoNode({ data }: NodeProps) {
    const c = useCanvas();
    const d = data as { placed: PlacedNode };
    const { from, port, ref } = d.placed;
    return (
        <div className="group relative h-full w-full">
            <Anchors />
            {c.onRemoveGoto && from !== undefined && (
                <button
                    type="button"
                    aria-label="Remove this go to"
                    title="Remove this go to"
                    onClick={(e) => {
                        e.stopPropagation();
                        c.setHoverGoto(null);
                        c.onRemoveGoto?.(from, port ?? "", ref);
                    }}
                    className="nodrag nopan absolute -right-2 -top-2 z-10 inline-flex size-5 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-400 opacity-100 shadow-sm transition-opacity hover:border-rose-200 hover:bg-rose-50 hover:text-rose-600 md:opacity-0 md:group-hover:opacity-100"
                >
                    <XIcon className="size-3" />
                </button>
            )}
            <button
                type="button"
                onClick={(e) => {
                    e.stopPropagation();
                    c.onSelect(d.placed.ref);
                    c.reveal(d.placed.ref);
                }}
                onMouseEnter={() => c.setHoverGoto(d.placed.ref)}
                onMouseLeave={() => c.setHoverGoto(null)}
                onFocus={() => c.setHoverGoto(d.placed.ref)}
                onBlur={() => c.setHoverGoto(null)}
                title="Contacts on this path continue at that step"
                className="nodrag nopan flex h-full w-full items-center gap-1.5 rounded-full border border-dashed border-slate-300 bg-white px-3 text-[11.5px] text-slate-500 transition-colors hover:border-sky-300 hover:bg-sky-50/60 hover:text-sky-700"
            >
                <CornerDownRightIcon className="size-3.5 shrink-0" />
                <span className="shrink-0">Go to</span>
                <span className="min-w-0 truncate font-medium text-slate-700">{c.stepTitle(d.placed.ref)}</span>
            </button>
        </div>
    );
}

function AddNode({ data }: NodeProps) {
    const c = useCanvas();
    const d = data as { placed: PlacedNode };
    const at: InsertPoint = { from: d.placed.ref, port: d.placed.port ?? "", before: null };
    const active = samePoint(c.activeInsert, at);
    return (
        <div className="h-full w-full">
            <Anchors />
            {c.onInsert ? (
                <button
                    type="button"
                    aria-label="Add a step here"
                    title="Add a step"
                    onClick={(e) => {
                        e.stopPropagation();
                        c.onInsert?.(at, e.currentTarget.getBoundingClientRect());
                    }}
                    className={cn(
                        "nodrag nopan flex h-full w-full items-center justify-center rounded-full border border-dashed transition-colors",
                        active
                            ? "border-sky-400 bg-sky-50 text-sky-600"
                            : "border-slate-300 bg-white text-slate-400 hover:border-sky-400 hover:bg-sky-50 hover:text-sky-600",
                    )}
                >
                    <PlusIcon className="size-3.5" />
                </button>
            ) : (
                <span className="mx-auto mt-2.5 block size-2 rounded-full bg-slate-300" aria-label="End of path" />
            )}
        </div>
    );
}

const nodeTypes = { step: StepNode, goto: GotoNode, add: AddNode };

const PILL: Record<BranchTone, string> = {
    neutral: "border-slate-200 bg-white text-slate-600",
    yes: "border-emerald-200 bg-emerald-50 text-emerald-700",
    no: "border-slate-200 bg-slate-50 text-slate-600",
    case: "border-violet-200 bg-violet-50 text-violet-700",
    error: "border-rose-200 bg-rose-50 text-rose-700",
    warn: "border-amber-200 bg-amber-50 text-amber-700",
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

function TreeEdge({ data }: EdgeProps) {
    const c = useCanvas();
    const e = (data as { placed: PlacedEdge }).placed;
    const tone = e.tone ?? "neutral";
    const stroke = tone === "error" ? "var(--wb-edge-rose)" : "var(--wb-edge-faint)";
    const branch = e.busY !== null;
    const pillY = branch ? e.busY! + 26 : null;
    const plusY = branch ? e.ty - 20 : (e.sy + e.ty) / 2;
    const active = samePoint(c.activeInsert, e.insert);
    return (
        <>
            <BaseEdge path={edgePath(e)} style={{ stroke, strokeWidth: 1.5 }} />
            <EdgeLabelRenderer>
                {e.label && pillY !== null && (
                    <div
                        className={cn(
                            "nodrag nopan pointer-events-none absolute max-w-[180px] truncate rounded-full border px-2 py-0.5 text-[11px] font-medium leading-4",
                            PILL[tone],
                        )}
                        style={{ transform: `translate(-50%, -50%) translate(${e.tx}px, ${pillY}px)` }}
                        title={e.label}
                    >
                        {e.label}
                    </div>
                )}
                {c.onInsert && e.insert && (
                    <button
                        type="button"
                        aria-label="Insert a step here"
                        title="Insert a step"
                        onClick={(ev) => {
                            ev.stopPropagation();
                            c.onInsert?.(e.insert!, ev.currentTarget.getBoundingClientRect());
                        }}
                        className={cn(
                            "nodrag nopan pointer-events-auto absolute flex size-5 items-center justify-center rounded-full border shadow-sm transition-[colors,transform] hover:scale-110",
                            active
                                ? "scale-110 border-sky-400 bg-sky-50 text-sky-600"
                                : "border-slate-200 bg-white text-slate-400 hover:border-sky-400 hover:bg-sky-50 hover:text-sky-600",
                        )}
                        style={{ transform: `translate(-50%, -50%) translate(${e.tx}px, ${plusY}px)` }}
                    >
                        <PlusIcon className="size-3" />
                    </button>
                )}
            </EdgeLabelRenderer>
        </>
    );
}

const edgeTypes = { tree: TreeEdge };

// The dashed line from a hovered go-to chip to the step it jumps to.
function GotoLink({ layout, target }: { layout: FlowLayout; target: string | null }) {
    if (!target) return null;
    const to = layout.nodes.find((n) => n.kind === "step" && n.ref === target);
    if (!to) return null;
    const chips = layout.nodes.filter((n) => n.kind === "goto" && n.ref === target);
    return (
        <svg className="pointer-events-none absolute left-0 top-0 overflow-visible" width={1} height={1}>
            {chips.map((g) => {
                const right = to.x + to.w / 2 > g.x + g.w / 2;
                const sx = right ? g.x + g.w : g.x;
                const sy = g.y + g.h / 2;
                const tx = right ? to.x : to.x + to.w;
                const ty = to.y + to.h / 2;
                const bend = Math.max(60, Math.abs(tx - sx) / 2);
                const d = `M${sx},${sy} C${sx + (right ? bend : -bend)},${sy} ${tx + (right ? -bend : bend)},${ty} ${tx},${ty}`;
                return <path key={g.key} d={d} fill="none" stroke="var(--wb-edge-sky)" strokeWidth={1.5} strokeDasharray="5 4" />;
            })}
        </svg>
    );
}

// First view: the whole flow when it fits at a readable zoom, otherwise the
// top of it at a readable zoom.
function initialViewport(layout: FlowLayout, w: number, h: number) {
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const n of layout.nodes) {
        minX = Math.min(minX, n.x);
        minY = Math.min(minY, n.y);
        maxX = Math.max(maxX, n.x + n.w);
        maxY = Math.max(maxY, n.y + n.h);
    }
    const pad = 56;
    const bw = maxX - minX;
    const bh = maxY - minY;
    const fit = Math.min((w - pad * 2) / Math.max(bw, 1), (h - pad * 2) / Math.max(bh, 1));
    const zoom = Math.max(0.55, Math.min(1, fit));
    const root = layout.nodes[0];
    const cx = bw * zoom <= w - pad * 2 ? (minX + maxX) / 2 : root.x + root.w / 2;
    const y = bh * zoom <= h - pad * 2 ? (h - bh * zoom) / 2 - minY * zoom : pad - minY * zoom;
    return { x: w / 2 - cx * zoom, y, zoom };
}

function ViewportController({ layout, revealId, register }: { layout: FlowLayout; revealId?: string | null; register: (fn: (id: string) => void) => void }) {
    const rf = useReactFlow();
    const store = useStoreApi();
    const width = useStore((s) => s.width);
    const height = useStore((s) => s.height);
    const placedOnce = React.useRef(false);
    const layoutRef = React.useRef(layout);
    layoutRef.current = layout;

    React.useEffect(() => {
        if (placedOnce.current || !width || !height || layout.nodes.length === 0) return;
        placedOnce.current = true;
        void rf.setViewport(initialViewport(layout, width, height));
    }, [rf, layout, width, height]);

    const reveal = React.useCallback(
        (id: string) => {
            const n = layoutRef.current.nodes.find((p) => p.kind === "step" && p.ref === id);
            if (!n) return;
            const { x, y, zoom } = rf.getViewport();
            const { width: w, height: h } = store.getState();
            const left = n.x * zoom + x;
            const top = n.y * zoom + y;
            const right = left + n.w * zoom;
            const bottom = top + n.h * zoom;
            const margin = 40;
            if (left >= margin && top >= margin && right <= w - margin && bottom <= h - margin) return;
            void rf.setCenter(n.x + n.w / 2, n.y + n.h / 2, { zoom, duration: 320 });
        },
        [rf, store],
    );

    React.useEffect(() => register(reveal), [register, reveal]);
    React.useEffect(() => {
        if (!revealId || !placedOnce.current) return;
        // After a side panel opening has narrowed the canvas.
        const t = window.setTimeout(() => reveal(revealId), 220);
        return () => window.clearTimeout(t);
    }, [revealId, reveal]);
    return null;
}

function ZoomControls() {
    const rf = useReactFlow();
    const { zoom } = useViewport();
    const btn = "inline-flex size-7 items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900";
    return (
        <div className="flex items-center gap-0.5 rounded-lg border border-slate-200 bg-white p-0.5 shadow-sm">
            <button type="button" className={btn} aria-label="Zoom out" title="Zoom out" onClick={() => void rf.zoomOut({ duration: 160 })}>
                <MinusIcon className="size-3.5" />
            </button>
            <button
                type="button"
                className="h-7 min-w-11 rounded-md px-1 text-[11.5px] tabular-nums text-slate-600 transition-colors hover:bg-slate-100"
                title="Reset to 100%"
                onClick={() => void rf.zoomTo(1, { duration: 160 })}
            >
                {Math.round(zoom * 100)}%
            </button>
            <button type="button" className={btn} aria-label="Zoom in" title="Zoom in" onClick={() => void rf.zoomIn({ duration: 160 })}>
                <PlusIcon className="size-3.5" />
            </button>
            <span className="mx-0.5 h-4 w-px bg-slate-200" />
            <button
                type="button"
                className={btn}
                aria-label="Fit the whole flow"
                title="Fit the whole flow"
                onClick={() => void rf.fitView({ padding: 0.15, maxZoom: 1, duration: 240 })}
            >
                <MaximizeIcon className="size-3.5" />
            </button>
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
    activeInsert,
    dimmed,
    revealId,
    onInit,
    onSelectionIds,
    children,
}: FlowCanvasProps) {
    const [hoverGoto, setHoverGoto] = React.useState<string | null>(null);
    const revealRef = React.useRef<(id: string) => void>(() => {});
    const register = React.useCallback((fn: (id: string) => void) => {
        revealRef.current = fn;
    }, []);

    const nodes: Node[] = React.useMemo(
        () =>
            layout.nodes.map((p) => ({
                id: p.key,
                type: p.kind,
                position: { x: p.x, y: p.y },
                width: p.w,
                height: p.h,
                measured: { width: p.w, height: p.h },
                style: { width: p.w, height: p.h },
                data: { placed: p },
                draggable: false,
                connectable: false,
                selectable: p.kind === "step",
                focusable: p.kind === "step",
                selected: p.kind === "step" && p.key === selectedId,
            })),
        [layout, selectedId],
    );
    const edges: Edge[] = React.useMemo(
        () =>
            layout.edges.map((e) => ({
                id: e.key,
                source: e.from,
                target: e.to,
                type: "tree",
                data: { placed: e },
                selectable: false,
                focusable: false,
            })),
        [layout],
    );

    const ctx: CanvasCtx = {
        renderStep,
        stepTitle,
        onSelect,
        onInsert,
        onRemoveGoto,
        activeInsert,
        dimmed,
        selectedId,
        hoverGoto,
        setHoverGoto,
        reveal: (id) => revealRef.current(id),
    };

    return (
        <Ctx.Provider value={ctx}>
            <ReactFlow
                nodes={nodes}
                edges={edges}
                nodeTypes={nodeTypes}
                edgeTypes={edgeTypes}
                onInit={onInit}
                onNodeClick={(_, n) => {
                    if (n.type === "step") onSelect(n.id);
                }}
                onPaneClick={() => onSelect(null)}
                onSelectionChange={onSelectionIds ? ({ nodes: sel }) => onSelectionIds(sel.map((n) => n.id)) : undefined}
                nodesDraggable={false}
                nodesConnectable={false}
                edgesFocusable={false}
                elementsSelectable
                selectNodesOnDrag={false}
                deleteKeyCode={null}
                multiSelectionKeyCode={null}
                selectionKeyCode={null}
                panOnScroll
                zoomOnScroll={false}
                zoomOnPinch
                zoomOnDoubleClick={false}
                minZoom={0.25}
                maxZoom={1.5}
                proOptions={{ hideAttribution: true }}
            >
                <Background color="light-dark(#e2e8f0, #26272b)" gap={20} size={1.2} />
                <ViewportController layout={layout} revealId={revealId} register={register} />
                <ViewportPortal>
                    <GotoLink layout={layout} target={hoverGoto} />
                </ViewportPortal>
                <Panel position="bottom-right">
                    <ZoomControls />
                </Panel>
                {children}
            </ReactFlow>
        </Ctx.Provider>
    );
}
