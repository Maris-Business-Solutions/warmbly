// FlowCanvas is the surface a flow tree is drawn on, behaving like a design
// tool's canvas: two-finger scroll or a drag on empty space pans (with a little
// momentum), pinch or Ctrl/⌘ + wheel zooms toward the pointer, and the dot grid
// moves with the view. No scrollbars. The flow can never be panned out of
// sight. It owns nothing about what the steps mean: the builder hands it the
// layout, a renderer for each step card, and what each button does.

import React from "react";
import { ClockIcon, CornerDownRightIcon, MaximizeIcon, MinusIcon, PlusIcon, XIcon } from "lucide-react";
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
    // Glide this step into view whenever it changes, if it is hidden.
    revealId?: string | null;
    // Teammates on the same flow, in flow coordinates.
    cursors?: RemoteCursor[];
    selections?: RemoteSelection[];
    // Our own pointer in flow coordinates, null when it leaves.
    onCursor?: (p: { x: number; y: number } | null) => void;
    // Floating controls over the flow (see FlowOverlay).
    children?: React.ReactNode;
}

type Viewport = { x: number; y: number; zoom: number };

const MIN_ZOOM = 0.3;
const MAX_ZOOM = 2;
// How much of the flow, in screen px, always stays in view.
const KEEP_VISIBLE = 140;
const GRID = 20;
const DRAG_SLOP = 4;
const MARGIN = 56;

const samePoint = (a?: InsertPoint | null, b?: InsertPoint | null) =>
    !!a && !!b && a.from === b.from && a.port === b.port && a.before === b.before;

const clampZoom = (z: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z));

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

const INTERACTIVE = "button, a, input, textarea, select, [role='button'], [contenteditable='true'], [data-no-pan]";

// A floating control over the flow that stays put while the view moves.
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
    children,
}: FlowCanvasProps) {
    const [hoverGoto, setHoverGoto] = React.useState<string | null>(null);
    const rootRef = React.useRef<HTMLDivElement>(null);
    const layerRef = React.useRef<HTMLDivElement>(null);

    const bounds = React.useMemo(() => {
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
        if (!layout.nodes.length) minX = minY = maxX = maxY = 0;
        return { minX, minY, maxX, maxY };
    }, [layout]);
    const boundsRef = React.useRef(bounds);
    boundsRef.current = bounds;

    // ── The viewport ─────────────────────────────────────────────────────────
    // Kept in a ref and written straight to the DOM, so panning never
    // re-renders the cards; the zoom readout subscribes on its own.
    const vp = React.useRef<Viewport>({ x: 0, y: 0, zoom: 1 });
    const listeners = React.useRef(new Set<() => void>());
    const anim = React.useRef<number | null>(null);
    const [ready, setReady] = React.useState(false);

    const clamp = React.useCallback((v: Viewport): Viewport => {
        const el = rootRef.current;
        if (!el) return v;
        const b = boundsRef.current;
        const w = el.clientWidth;
        const h = el.clientHeight;
        const zoom = clampZoom(v.zoom);
        const keepX = Math.min(KEEP_VISIBLE, ((b.maxX - b.minX) * zoom) / 2 + 40);
        const keepY = Math.min(KEEP_VISIBLE, ((b.maxY - b.minY) * zoom) / 2 + 40);
        const x = Math.min(w - keepX - b.minX * zoom, Math.max(keepX - b.maxX * zoom, v.x));
        const y = Math.min(h - keepY - b.minY * zoom, Math.max(keepY - b.maxY * zoom, v.y));
        return { x, y, zoom };
    }, []);

    const paint = React.useCallback(() => {
        const { x, y, zoom } = vp.current;
        const layer = layerRef.current;
        const root = rootRef.current;
        if (layer) {
            layer.style.transform = `translate3d(${x}px, ${y}px, 0) scale(${zoom})`;
            layer.style.setProperty("--inv-zoom", String(1 / zoom));
        }
        if (root) {
            root.style.backgroundPosition = `${x}px ${y}px`;
            root.style.backgroundSize = `${GRID * zoom}px ${GRID * zoom}px`;
        }
        listeners.current.forEach((f) => f());
    }, []);

    const stopAnim = () => {
        if (anim.current !== null) cancelAnimationFrame(anim.current);
        anim.current = null;
    };

    const set = React.useCallback(
        (next: Viewport, animate = false) => {
            stopAnim();
            const to = clamp(next);
            if (!animate) {
                vp.current = to;
                paint();
                return;
            }
            const from = { ...vp.current };
            const start = performance.now();
            const step = (now: number) => {
                const t = Math.min(1, (now - start) / 260);
                const k = 1 - Math.pow(1 - t, 3);
                vp.current = { x: from.x + (to.x - from.x) * k, y: from.y + (to.y - from.y) * k, zoom: from.zoom + (to.zoom - from.zoom) * k };
                paint();
                anim.current = t < 1 ? requestAnimationFrame(step) : null;
            };
            anim.current = requestAnimationFrame(step);
        },
        [clamp, paint],
    );

    const zoomAt = React.useCallback(
        (px: number, py: number, zoom: number, animate = false) => {
            const { x, y, zoom: z } = vp.current;
            const z2 = clampZoom(zoom);
            set({ x: px - ((px - x) * z2) / z, y: py - ((py - y) * z2) / z, zoom: z2 }, animate);
        },
        [set],
    );

    const fit = React.useCallback(
        (animate = true, topAnchored = false) => {
            const el = rootRef.current;
            if (!el) return;
            const b = boundsRef.current;
            const w = el.clientWidth;
            const h = el.clientHeight;
            const bw = Math.max(b.maxX - b.minX, 1);
            const bh = Math.max(b.maxY - b.minY, 1);
            const fitZoom = Math.min((w - MARGIN * 2) / bw, (h - MARGIN * 2) / bh);
            // Never zoom past 100% to fit a small flow.
            // A first view nearly at 100% opens at a crisp 100%.
            const zoom = clampZoom(Math.min(1, topAnchored ? (fitZoom >= 0.85 ? 1 : Math.max(0.6, fitZoom)) : fitZoom));
            const root = layout.nodes[0];
            const fitsWide = bw * zoom <= w - MARGIN * 2;
            const cx = fitsWide || !root ? (b.minX + b.maxX) / 2 : root.x + root.w / 2;
            const fitsTall = bh * zoom <= h - MARGIN * 2;
            const y = topAnchored || !fitsTall ? MARGIN - b.minY * zoom : (h - bh * zoom) / 2 - b.minY * zoom;
            set({ x: w / 2 - cx * zoom, y, zoom }, animate);
        },
        [layout, set],
    );

    // First view: framed from the top once the surface has a size.
    React.useLayoutEffect(() => {
        const el = rootRef.current;
        if (!el) return;
        const place = () => {
            if (ready || !el.clientWidth || !el.clientHeight || !layout.nodes.length) return;
            fit(false, true);
            setReady(true);
        };
        place();
        const ro = new ResizeObserver(() => (ready ? set(vp.current) : place()));
        ro.observe(el);
        return () => ro.disconnect();
    }, [fit, layout.nodes.length, ready, set]);

    // The flow changed shape (a step added or removed): stay where we are, but
    // never past the new edges.
    React.useEffect(() => {
        if (ready) set(vp.current);
    }, [bounds, ready, set]);

    React.useEffect(() => () => stopAnim(), []);

    // ── Input ────────────────────────────────────────────────────────────────
    React.useEffect(() => {
        const el = rootRef.current;
        if (!el) return;
        const onWheel = (e: WheelEvent) => {
            if ((e.target as Element | null)?.closest?.("[data-floating], [data-wheel-scroll]")) return;
            e.preventDefault();
            stopAnim();
            const r = el.getBoundingClientRect();
            const unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? el.clientHeight : 1;
            if (e.ctrlKey || e.metaKey) {
                // Trackpad pinch arrives as ctrl + wheel with small deltas.
                const factor = Math.exp((-e.deltaY * unit) / 240);
                zoomAt(e.clientX - r.left, e.clientY - r.top, vp.current.zoom * factor);
                return;
            }
            let dx = e.deltaX * unit;
            let dy = e.deltaY * unit;
            if (e.shiftKey && !dx) {
                dx = dy;
                dy = 0;
            }
            set({ ...vp.current, x: vp.current.x - dx, y: vp.current.y - dy });
        };
        el.addEventListener("wheel", onWheel, { passive: false });
        // Safari's own pinch gesture would zoom the whole page.
        const stopGesture = (e: Event) => e.preventDefault();
        el.addEventListener("gesturestart", stopGesture);
        el.addEventListener("gesturechange", stopGesture);
        return () => {
            el.removeEventListener("wheel", onWheel);
            el.removeEventListener("gesturestart", stopGesture);
            el.removeEventListener("gesturechange", stopGesture);
        };
    }, [set, zoomAt]);

    // Space held: drag anywhere to pan, as in design tools.
    const [spaceHeld, setSpaceHeld] = React.useState(false);
    const hovered = React.useRef(false);
    React.useEffect(() => {
        const typing = (t: EventTarget | null) => {
            const n = t as HTMLElement | null;
            return !!n && (n.tagName === "INPUT" || n.tagName === "TEXTAREA" || n.isContentEditable);
        };
        const down = (e: KeyboardEvent) => {
            if (typing(e.target) || !hovered.current) return;
            if (e.code === "Space" && !e.repeat) {
                e.preventDefault();
                setSpaceHeld(true);
            }
            const el = rootRef.current;
            if (!el) return;
            const mod = e.metaKey || e.ctrlKey;
            const cx = el.clientWidth / 2;
            const cy = el.clientHeight / 2;
            if (mod && (e.key === "=" || e.key === "+")) {
                e.preventDefault();
                zoomAt(cx, cy, vp.current.zoom * 1.25, true);
            } else if (mod && e.key === "-") {
                e.preventDefault();
                zoomAt(cx, cy, vp.current.zoom / 1.25, true);
            } else if (mod && e.key === "0") {
                e.preventDefault();
                zoomAt(cx, cy, 1, true);
            } else if (e.shiftKey && e.code === "Digit1") {
                e.preventDefault();
                fit(true);
            }
        };
        const up = (e: KeyboardEvent) => {
            if (e.code === "Space") setSpaceHeld(false);
        };
        window.addEventListener("keydown", down);
        window.addEventListener("keyup", up);
        return () => {
            window.removeEventListener("keydown", down);
            window.removeEventListener("keyup", up);
        };
    }, [fit, zoomAt]);

    // Drag to pan (one pointer), pinch to zoom (two touch pointers).
    const pointers = React.useRef(new Map<number, { x: number; y: number }>());
    const drag = React.useRef<{ x: number; y: number; moved: boolean; vx: number; vy: number; t: number } | null>(null);
    const pinch = React.useRef<{ dist: number; mx: number; my: number } | null>(null);
    const suppressClick = React.useRef(false);
    const [grabbing, setGrabbing] = React.useState(false);

    const local = (e: { clientX: number; clientY: number }) => {
        const r = rootRef.current!.getBoundingClientRect();
        return { x: e.clientX - r.left, y: e.clientY - r.top };
    };

    const onPointerDown = (e: React.PointerEvent) => {
        if (e.button !== 0 && e.pointerType === "mouse" && e.button !== 1) return;
        const onControl = !!(e.target as HTMLElement).closest(INTERACTIVE);
        if (onControl && !spaceHeld && e.button !== 1) return;
        stopAnim();
        const p = local(e);
        pointers.current.set(e.pointerId, p);
        rootRef.current?.setPointerCapture(e.pointerId);
        if (pointers.current.size === 2) {
            const [a, b] = [...pointers.current.values()];
            pinch.current = { dist: Math.hypot(a.x - b.x, a.y - b.y), mx: (a.x + b.x) / 2, my: (a.y + b.y) / 2 };
            drag.current = null;
            return;
        }
        drag.current = { x: p.x, y: p.y, moved: false, vx: 0, vy: 0, t: performance.now() };
    };

    const onPointerMove = (e: React.PointerEvent) => {
        if (onCursor) {
            const p = local(e);
            const { x, y, zoom } = vp.current;
            onCursor({ x: (p.x - x) / zoom, y: (p.y - y) / zoom });
        }
        if (!pointers.current.has(e.pointerId)) return;
        const p = local(e);
        pointers.current.set(e.pointerId, p);
        if (pinch.current && pointers.current.size === 2) {
            const [a, b] = [...pointers.current.values()];
            const dist = Math.hypot(a.x - b.x, a.y - b.y);
            const mx = (a.x + b.x) / 2;
            const my = (a.y + b.y) / 2;
            const { x, y, zoom } = vp.current;
            const z2 = clampZoom((zoom * dist) / pinch.current.dist);
            set({ x: mx - ((pinch.current.mx - x) * z2) / zoom, y: my - ((pinch.current.my - y) * z2) / zoom, zoom: z2 });
            pinch.current = { dist, mx, my };
            suppressClick.current = true;
            return;
        }
        const d = drag.current;
        if (!d) return;
        const dx = p.x - d.x;
        const dy = p.y - d.y;
        if (!d.moved && Math.hypot(dx, dy) < DRAG_SLOP) return;
        if (!d.moved) {
            d.moved = true;
            setGrabbing(true);
        }
        const now = performance.now();
        const dt = Math.max(1, now - d.t);
        d.vx = 0.8 * (dx / dt) + 0.2 * d.vx;
        d.vy = 0.8 * (dy / dt) + 0.2 * d.vy;
        d.t = now;
        d.x = p.x;
        d.y = p.y;
        set({ ...vp.current, x: vp.current.x + dx, y: vp.current.y + dy });
    };

    const onPointerUp = (e: React.PointerEvent) => {
        pointers.current.delete(e.pointerId);
        if (pointers.current.size < 2) pinch.current = null;
        const d = drag.current;
        if (pointers.current.size > 0) return;
        drag.current = null;
        setGrabbing(false);
        if (!d?.moved) return;
        suppressClick.current = true;
        // A short glide in the direction of the throw.
        let vx = d.vx * 16;
        let vy = d.vy * 16;
        if (Math.hypot(vx, vy) < 2 || performance.now() - d.t > 80) return;
        const glide = () => {
            vx *= 0.92;
            vy *= 0.92;
            if (Math.hypot(vx, vy) < 0.3) {
                anim.current = null;
                return;
            }
            vp.current = clamp({ ...vp.current, x: vp.current.x + vx, y: vp.current.y + vy });
            paint();
            anim.current = requestAnimationFrame(glide);
        };
        anim.current = requestAnimationFrame(glide);
    };

    // Clicking empty canvas clears the selection, but a pan never does.
    const onClickCapture = (e: React.MouseEvent) => {
        if (!suppressClick.current) return;
        suppressClick.current = false;
        e.stopPropagation();
        e.preventDefault();
    };

    // Glide a step into view only when it is hidden (behind a panel, off screen).
    const reveal = React.useCallback(
        (id: string) => {
            const el = rootRef.current;
            const n = layout.nodes.find((p) => p.kind === "step" && p.ref === id);
            if (!el || !n) return;
            const { x, y, zoom } = vp.current;
            const w = el.clientWidth;
            const h = el.clientHeight;
            const left = n.x * zoom + x;
            const top = n.y * zoom + y;
            const right = left + n.w * zoom;
            const bottom = top + n.h * zoom;
            const m = 32;
            const dx = left < m ? m - left : right > w - m ? Math.max(m - left, w - m - right) : 0;
            const dy = top < m ? m - top : bottom > h - m ? Math.max(m - top, h - m - bottom) : 0;
            if (dx || dy) set({ x: x + dx, y: y + dy, zoom }, true);
        },
        [layout, set],
    );
    React.useEffect(() => {
        if (!revealId || !ready) return;
        // After a side panel opening has narrowed the surface.
        const t = window.setTimeout(() => reveal(revealId), 200);
        return () => window.clearTimeout(t);
    }, [revealId, ready, reveal]);

    // ── Drawing ──────────────────────────────────────────────────────────────
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
                style={{ left: p.x, top: p.y, width: p.w, height: p.h }}
            >
                {renderStep(p.key, selected, !!p.detached)}
            </div>
        );
    };

    const gotoNode = (p: PlacedNode) => (
        <div key={p.key} className="group absolute" style={{ left: p.x, top: p.y, width: p.w, height: p.h }}>
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
            <div key={p.key} className="absolute" style={{ left: p.x, top: p.y, width: p.w, height: p.h }}>
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
                        style={at(e.tx, pillY)}
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
                        style={{ left: e.tx + 14, top: midY, transform: "translateY(-50%)" }}
                    >
                        <ClockIcon className="size-3" />
                        {e.note}
                    </button>
                )}
                {onInsert && e.insert && (
                    // A hover zone over the line: the + shows only there (always on touch).
                    <div className="group/ins absolute flex w-8 items-center justify-center" style={{ ...at(e.tx, midY), height: Math.max(20, e.ty - top - 8) }}>
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
                const sx = right ? g.x + g.w : g.x;
                const sy = g.y + g.h / 2;
                const tx = right ? to.x : to.x + to.w;
                const ty = to.y + to.h / 2;
                const bend = Math.max(60, Math.abs(tx - sx) / 2);
                const d = `M${sx},${sy} C${sx + (right ? bend : -bend)},${sy} ${tx + (right ? -bend : bend)},${ty} ${tx},${ty}`;
                return <path key={g.key} d={d} fill="none" stroke="var(--wb-edge-sky)" strokeWidth={1.5} strokeDasharray="5 4" />;
            });
    };

    const stepByKey = new Map(layout.nodes.filter((n) => n.kind === "step").map((n) => [n.key, n]));

    return (
        <div
            ref={rootRef}
            className={cn(
                "relative h-full w-full touch-none select-none overflow-hidden overscroll-none",
                spaceHeld || grabbing ? (grabbing ? "cursor-grabbing" : "cursor-grab") : "cursor-default",
            )}
            style={{ backgroundImage: "radial-gradient(circle, var(--flow-dot) 1px, transparent 1.2px)" }}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={onPointerUp}
            onPointerEnter={() => (hovered.current = true)}
            onPointerLeave={() => {
                hovered.current = false;
                onCursor?.(null);
            }}
            onClickCapture={onClickCapture}
            onClick={() => onSelect(null)}
        >
            <div ref={layerRef} className={cn("absolute left-0 top-0 origin-top-left will-change-transform", !ready && "invisible")}>
                <svg className="pointer-events-none absolute left-0 top-0 overflow-visible" width={1} height={1} aria-hidden="true">
                    {layout.edges.map((e) => (
                        <path key={e.key} d={edgePath(e)} fill="none" stroke={e.tone === "error" ? "var(--wb-edge-rose)" : "var(--wb-edge-faint)"} strokeWidth={1.25} vectorEffect="non-scaling-stroke" />
                    ))}
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
                            style={{ left: n.x - pad, top: n.y - pad, width: n.w + pad * 2, height: n.h + pad * 2, borderColor: s.color }}
                        >
                            {n.key === tag.key && (
                                <span className="absolute -top-[19px] left-0 max-w-[160px] truncate rounded px-1.5 py-px text-[10px] font-medium leading-4 text-white" style={{ backgroundColor: s.color }}>
                                    {s.name ?? "Teammate"}
                                </span>
                            )}
                        </div>
                    ));
                })}
                {(cursors ?? []).map((c) => (
                    // Counter-scaled so a teammate's pointer stays the same size at any zoom.
                    <div key={c.userId} className="pointer-events-none absolute z-20 origin-top-left" style={{ left: c.x, top: c.y, transform: "scale(var(--inv-zoom, 1))" }}>
                        <Cursor color={c.color} name={c.name} avatar={c.avatar} chat={c.chat} left={0} top={0} />
                    </div>
                ))}
            </div>
            <ZoomControls
                subscribe={(f) => {
                    listeners.current.add(f);
                    return () => listeners.current.delete(f);
                }}
                zoom={() => vp.current.zoom}
                onZoom={(factor) => {
                    const el = rootRef.current;
                    if (el) zoomAt(el.clientWidth / 2, el.clientHeight / 2, factor === 0 ? 1 : vp.current.zoom * factor, true);
                }}
                onFit={() => fit(true)}
            />
            {children}
        </div>
    );
}

function ZoomControls({
    subscribe,
    zoom,
    onZoom,
    onFit,
}: {
    subscribe: (f: () => void) => () => void;
    zoom: () => number;
    onZoom: (factor: number) => void;
    onFit: () => void;
}) {
    const value = React.useSyncExternalStore(subscribe, zoom);
    const btn = "inline-flex size-7 items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900";
    return (
        <div
            data-no-pan
            onClick={(e) => e.stopPropagation()}
            className="absolute bottom-3 right-3 z-10 flex items-center gap-0.5 rounded-lg border border-slate-200 bg-white/95 p-0.5 shadow-sm backdrop-blur"
        >
            <button type="button" className={btn} aria-label="Zoom out" title="Zoom out (⌘−)" onClick={() => onZoom(1 / 1.25)}>
                <MinusIcon className="size-3.5" />
            </button>
            <button type="button" className="h-7 min-w-11 rounded-md px-1 text-[11.5px] tabular-nums text-slate-600 transition-colors hover:bg-slate-100" title="Reset to 100% (⌘0)" onClick={() => onZoom(0)}>
                {Math.round(value * 100)}%
            </button>
            <button type="button" className={btn} aria-label="Zoom in" title="Zoom in (⌘+)" onClick={() => onZoom(1.25)}>
                <PlusIcon className="size-3.5" />
            </button>
            <span className="mx-0.5 h-4 w-px bg-slate-200" />
            <button type="button" className={btn} aria-label="Fit the whole flow" title="Fit the whole flow (⇧1)" onClick={onFit}>
                <MaximizeIcon className="size-3.5" />
            </button>
        </div>
    );
}
