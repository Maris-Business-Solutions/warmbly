// Tree layout for the flow builders. A graph is drawn as a top-down tree: every
// node is placed once (where it is first reached), branches sit side by side
// under their node, and any other path into an already-placed node ends in a
// "go to" chip instead of a crossing line. Positions are always computed, never
// stored, so two people looking at the same graph see the same picture.

export type BranchTone = "neutral" | "yes" | "no" | "case" | "error" | "warn";

// One outgoing port of a node, in display order.
export interface FlowPort {
    port: string;
    // A labelled port is drawn as a column under a pill; an unlabelled single
    // port continues the chain straight down.
    label?: string;
    tone?: BranchTone;
    targets: string[];
    // Draw the column (with an add button) even while nothing is connected.
    showEmpty?: boolean;
    // No insert button on this port's lines (an insert the data cannot express).
    noInsert?: boolean;
}

export interface FlowSource {
    roots: string[];
    nodeIds: string[];
    ports: (id: string) => FlowPort[];
    size: (id: string) => { w: number; h: number };
    // The port a trailing add button appends to, or null for a node that ends
    // its path (a stop).
    appendPort: (id: string) => string | null;
    // A short label drawn beside the line into `target` (a wait before it).
    note?: (from: string, port: string, target: string) => string | undefined;
}

export type PlacedKind = "step" | "goto" | "add";

export interface PlacedNode {
    key: string;
    kind: PlacedKind;
    x: number;
    y: number;
    w: number;
    h: number;
    // step: the node id. goto: the target id. add: the node the button appends to.
    ref: string;
    // goto / add: the port the item hangs from.
    port?: string;
    // goto: the step the chip hangs from.
    from?: string;
    // A root that is not one of the declared roots: nothing leads to it.
    detached?: boolean;
}

export interface InsertPoint {
    from: string;
    port: string;
    // The step the new node goes in front of; null appends at a branch's end.
    before: string | null;
}

export interface PlacedEdge {
    key: string;
    from: string;
    to: string;
    // Absolute geometry: leave the parent at (sx, sy), run along busY when the
    // edge is a branch, enter the child at (tx, ty).
    sx: number;
    sy: number;
    tx: number;
    ty: number;
    busY: number | null;
    label?: string;
    tone?: BranchTone;
    // Where the inline add button on this edge inserts; null when the child is
    // itself an add button.
    insert: InsertPoint | null;
    port: string;
    note?: string;
}

export interface FlowLayout {
    nodes: PlacedNode[];
    edges: PlacedEdge[];
    // Each step's parent edge, for reachability questions (ancestors, path).
    parentOf: Map<string, { from: string; port: string }>;
}

export const GOTO_W = 184;
export const GOTO_H = 26;
export const ADD_W = 22;
export const ADD_H = 22;
const CHAIN_GAP = 52;
const BRANCH_GAP = 92;
const BUS_DROP = 20;
const COL_GAP = 36;
const FOREST_GAP = 96;

interface Branch {
    port: string;
    label?: string;
    tone?: BranchTone;
    child: Item;
    note?: string;
    noInsert?: boolean;
}

type Item =
    | { kind: "step"; id: string; w: number; h: number; branches: Branch[]; chain: boolean; width: number; detached?: boolean }
    | { kind: "goto"; key: string; target: string; from: string; port: string; w: number; h: number; width: number }
    | { kind: "add"; key: string; from: string; port: string; w: number; h: number; width: number };

// A column is never narrower than the pill naming it (about 6.4px a character
// at 11px, capped like the pill itself), so neighbouring pills never overlap.
const colWidth = (b: Branch) => Math.max(b.child.width, b.label ? Math.min(200, b.label.length * 6.4 + 20) : 0);

export function layoutFlow(src: FlowSource): FlowLayout {
    const known = new Set(src.nodeIds);
    const visited = new Set<string>();
    const parentOf = new Map<string, { from: string; port: string }>();

    const build = (id: string, detached?: boolean): Item => {
        visited.add(id);
        const { w, h } = src.size(id);
        const branches: Branch[] = [];
        for (const p of src.ports(id)) {
            const targets = p.targets.filter((t) => known.has(t));
            if (targets.length === 0) {
                if (p.showEmpty) {
                    branches.push({
                        port: p.port,
                        label: p.label,
                        tone: p.tone,
                        child: { kind: "add", key: `add:${id}:${p.port}`, from: id, port: p.port, w: ADD_W, h: ADD_H, width: ADD_W },
                    });
                }
                continue;
            }
            targets.forEach((t, i) => {
                let child: Item;
                if (visited.has(t)) {
                    child = { kind: "goto", key: `goto:${id}:${p.port}:${t}:${i}`, target: t, from: id, port: p.port, w: GOTO_W, h: GOTO_H, width: GOTO_W };
                } else {
                    parentOf.set(t, { from: id, port: p.port });
                    child = build(t);
                }
                branches.push({ port: p.port, label: p.label, tone: p.tone, child, note: src.note?.(id, p.port, t), noInsert: p.noInsert });
            });
        }
        if (branches.length === 0) {
            const port = src.appendPort(id);
            if (port !== null) {
                branches.push({ port, child: { kind: "add", key: `add:${id}:${port}`, from: id, port, w: ADD_W, h: ADD_H, width: ADD_W } });
            }
        }
        // Several plain targets on one port run side by side; label them so the
        // columns read as parallel paths rather than a broken chain.
        const unlabelled = branches.filter((b) => !b.label);
        if (branches.length > 1 && unlabelled.length > 0) {
            let n = 0;
            for (const b of branches) if (!b.label) b.label = unlabelled.length > 1 ? `Path ${++n}` : "Then";
        }
        const chain = branches.length === 1 && !branches[0].label;
        const width = chain
            ? Math.max(w, branches[0].child.width)
            : Math.max(w, branches.reduce((s, b) => s + colWidth(b), 0) + COL_GAP * Math.max(0, branches.length - 1));
        return { kind: "step", id, w, h, branches, chain, width, detached };
    };

    const trees: Item[] = [];
    for (const r of src.roots) if (known.has(r) && !visited.has(r)) trees.push(build(r));
    // Nodes nothing reaches: start a tree at each one no other unplaced node
    // points at, then sweep up whatever is left (a detached cycle).
    const incoming = new Map<string, number>();
    for (const id of src.nodeIds) {
        if (visited.has(id)) continue;
        for (const p of src.ports(id)) for (const t of p.targets) if (!visited.has(t)) incoming.set(t, (incoming.get(t) ?? 0) + 1);
    }
    for (const id of src.nodeIds) if (!visited.has(id) && !incoming.get(id)) trees.push(build(id, true));
    for (const id of src.nodeIds) if (!visited.has(id)) trees.push(build(id, true));

    const nodes: PlacedNode[] = [];
    const edges: PlacedEdge[] = [];
    const keyOf = (it: Item) => (it.kind === "step" ? it.id : it.key);

    const place = (it: Item, cx: number, y: number) => {
        const x = cx - it.w / 2;
        if (it.kind === "goto") {
            nodes.push({ key: it.key, kind: "goto", x, y, w: it.w, h: it.h, ref: it.target, port: it.port, from: it.from });
            return;
        }
        if (it.kind === "add") {
            nodes.push({ key: it.key, kind: "add", x, y, w: it.w, h: it.h, ref: it.from, port: it.port });
            return;
        }
        nodes.push({ key: it.id, kind: "step", x, y, w: it.w, h: it.h, ref: it.id, detached: it.detached });
        const sy = y + it.h;
        if (it.chain) {
            const b = it.branches[0];
            const ty = sy + (b.child.kind === "add" ? CHAIN_GAP / 2 : CHAIN_GAP);
            edges.push(edgeFor(it.id, b, cx, sy, cx, ty, null));
            place(b.child, cx, ty);
            return;
        }
        const total = it.branches.reduce((s, b) => s + colWidth(b), 0) + COL_GAP * Math.max(0, it.branches.length - 1);
        let left = cx - total / 2;
        const ty = sy + BRANCH_GAP;
        for (const b of it.branches) {
            const ccx = left + colWidth(b) / 2;
            edges.push(edgeFor(it.id, b, cx, sy, ccx, ty, sy + BUS_DROP));
            place(b.child, ccx, ty);
            left += colWidth(b) + COL_GAP;
        }
    };

    const edgeFor = (from: string, b: Branch, sx: number, sy: number, tx: number, ty: number, busY: number | null): PlacedEdge => ({
        key: `e:${from}:${b.port}:${keyOf(b.child)}`,
        from,
        to: keyOf(b.child),
        sx,
        sy,
        tx,
        ty,
        busY,
        label: b.label,
        tone: b.tone,
        insert: b.child.kind === "add" || b.noInsert ? null : { from, port: b.port, before: b.child.kind === "step" ? b.child.id : b.child.target },
        port: b.port,
        note: b.note,
    });

    let left = 0;
    for (const t of trees) {
        place(t, left + t.width / 2, 0);
        left += t.width + FOREST_GAP;
    }
    return { nodes, edges, parentOf };
}

// Every node reachable from `start` along any edge, `start` included. A go-to
// from X to T loops exactly when X is in reachableFrom(T).
export function reachableFrom(src: Pick<FlowSource, "ports">, start: string): Set<string> {
    const out = new Set<string>([start]);
    const queue = [start];
    while (queue.length) {
        const id = queue.shift()!;
        for (const p of src.ports(id)) {
            for (const t of p.targets) {
                if (!out.has(t)) {
                    out.add(t);
                    queue.push(t);
                }
            }
        }
    }
    return out;
}
