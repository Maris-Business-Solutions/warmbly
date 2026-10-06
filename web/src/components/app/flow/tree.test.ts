import { describe, expect, it } from "vitest";
import { layoutFlow, reachableFrom, type FlowPort, type FlowSource } from "./tree";

// A tiny graph DSL: ports keyed by node, each port a list of targets.
function source(graph: Record<string, FlowPort[]>, roots = ["t"], stops: string[] = []): FlowSource {
    return {
        roots,
        nodeIds: Object.keys(graph),
        ports: (id) => graph[id] ?? [],
        size: () => ({ w: 200, h: 60 }),
        appendPort: (id) => (stops.includes(id) ? null : ""),
    };
}

describe("layoutFlow", () => {
    it("chains single plain ports straight down and ends in an add button", () => {
        const l = layoutFlow(source({ t: [{ port: "", targets: ["a"] }], a: [{ port: "", targets: [] }] }));
        const t = l.nodes.find((n) => n.key === "t")!;
        const a = l.nodes.find((n) => n.key === "a")!;
        expect(a.x).toBe(t.x);
        expect(a.y).toBeGreaterThan(t.y + t.h);
        const add = l.nodes.find((n) => n.kind === "add")!;
        expect(add.ref).toBe("a");
        expect(l.edges.find((e) => e.to === "a")!.insert).toEqual({ from: "t", port: "", before: "a" });
        expect(l.edges.find((e) => e.to === add.key)!.insert).toBeNull();
    });

    it("puts labelled branches side by side, keeping empty ones as add buttons", () => {
        const l = layoutFlow(
            source({
                t: [{ port: "", targets: ["c"] }],
                c: [
                    { port: "true", label: "Yes", tone: "yes", targets: ["a"], showEmpty: true },
                    { port: "false", label: "No", tone: "no", targets: [], showEmpty: true },
                ],
                a: [],
            }),
        );
        const a = l.nodes.find((n) => n.key === "a")!;
        const noAdd = l.nodes.find((n) => n.key === "add:c:false")!;
        expect(a.y).toBe(noAdd.y);
        expect(a.x + a.w / 2).toBeLessThan(noAdd.x + noAdd.w / 2);
        const yes = l.edges.find((e) => e.to === "a")!;
        expect(yes.label).toBe("Yes");
        expect(yes.busY).not.toBeNull();
    });

    it("draws a second path into a placed node as a go-to chip", () => {
        const l = layoutFlow(
            source({
                t: [{ port: "", targets: ["c"] }],
                c: [
                    { port: "true", label: "Yes", targets: ["a"], showEmpty: true },
                    { port: "false", label: "No", targets: ["a"], showEmpty: true },
                ],
                a: [],
            }),
        );
        expect(l.nodes.filter((n) => n.key === "a")).toHaveLength(1);
        const goto = l.nodes.find((n) => n.kind === "goto")!;
        expect(goto.ref).toBe("a");
        expect(goto.port).toBe("false");
        // Inserting on the go-to edge goes in front of the jump's target.
        expect(l.edges.find((e) => e.to === goto.key)!.insert).toEqual({ from: "c", port: "false", before: "a" });
    });

    it("turns a cycle back into a go-to instead of recursing", () => {
        const l = layoutFlow(source({ t: [{ port: "", targets: ["a"] }], a: [{ port: "", targets: ["t"] }] }));
        expect(l.nodes.find((n) => n.kind === "goto")?.ref).toBe("t");
    });

    it("labels parallel plain paths", () => {
        const l = layoutFlow(source({ t: [{ port: "", targets: ["a", "b"] }], a: [], b: [] }));
        expect(l.edges.filter((e) => e.from === "t").map((e) => e.label)).toEqual(["Path 1", "Path 2"]);
    });

    it("lays out unreachable nodes as detached trees to the right", () => {
        const l = layoutFlow(source({ t: [], x: [{ port: "", targets: ["y"] }], y: [] }));
        const x = l.nodes.find((n) => n.key === "x")!;
        expect(x.detached).toBe(true);
        expect(l.nodes.find((n) => n.key === "y")!.detached).toBeFalsy();
        expect(x.x).toBeGreaterThan(l.nodes.find((n) => n.key === "t")!.x);
    });

    it("gives a stop no trailing add button", () => {
        const l = layoutFlow(source({ t: [{ port: "", targets: ["s"] }], s: [] }, ["t"], ["s"]));
        expect(l.nodes.some((n) => n.kind === "add" && n.ref === "s")).toBe(false);
    });

    it("centres a parent over its columns", () => {
        const l = layoutFlow(
            source({
                t: [
                    { port: "a", label: "A", targets: ["a"] },
                    { port: "b", label: "B", targets: ["b"] },
                ],
                a: [],
                b: [],
            }),
        );
        const t = l.nodes.find((n) => n.key === "t")!;
        const a = l.nodes.find((n) => n.key === "a")!;
        const b = l.nodes.find((n) => n.key === "b")!;
        expect(t.x + t.w / 2).toBeCloseTo((a.x + a.w / 2 + b.x + b.w / 2) / 2);
    });
});

describe("reachableFrom", () => {
    it("follows every port", () => {
        const g: Record<string, FlowPort[]> = {
            a: [{ port: "true", targets: ["b"] }, { port: "false", targets: ["c"] }],
            b: [],
            c: [{ port: "", targets: ["d"] }],
            d: [],
        };
        expect([...reachableFrom({ ports: (id) => g[id] ?? [] }, "a")].sort()).toEqual(["a", "b", "c", "d"]);
    });
});

describe("notes and insert control", () => {
    it("makes room for a note on a line and carries it on the edge", () => {
        const g: Record<string, FlowPort[]> = { t: [{ port: "", targets: ["a"] }], a: [] };
        const plain = layoutFlow(source(g));
        const noted = layoutFlow({ ...source(g), note: (_f, _p, t) => (t === "a" ? "Wait 2 days" : undefined) });
        const ya = (l: typeof plain) => l.nodes.find((n) => n.key === "a")!.y;
        expect(ya(noted)).toBeGreaterThan(ya(plain));
        expect(noted.edges.find((e) => e.to === "a")!.note).toBe("Wait 2 days");
    });

    it("keeps columns level when only one branch has a note", () => {
        const g: Record<string, FlowPort[]> = {
            t: [
                { port: "x", label: "X", targets: ["a"] },
                { port: "y", label: "Y", targets: ["b"] },
            ],
            a: [],
            b: [],
        };
        const l = layoutFlow({ ...source(g), note: (_f, _p, t) => (t === "a" ? "Wait 1 day" : undefined) });
        expect(l.nodes.find((n) => n.key === "a")!.y).toBe(l.nodes.find((n) => n.key === "b")!.y);
    });

    it("drops the insert button on a no-insert port", () => {
        const l = layoutFlow(source({ t: [{ port: "", targets: ["a"], noInsert: true }], a: [] }));
        expect(l.edges.find((e) => e.to === "a")!.insert).toBeNull();
    });
});
