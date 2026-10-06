import { describe, expect, it } from "vitest";
import type { AutomationGraph, AutomationNode } from "@/lib/api/models/app/automations/Automation";
import {
    addGoto,
    gotoTargets,
    graphIssues,
    healForSave,
    insertNode,
    nodeIssue,
    nodePorts,
    normalizeGraph,
    removalImpact,
    renameCases,
} from "./automationGraph";

const node = (id: string, type: AutomationNode["type"], extra: Partial<AutomationNode> = {}): AutomationNode => ({ id, type, x: 0, y: 0, ...extra });
const tag = (id: string) => node(id, "action", { action: "warmbly.add_tag", config: { category_id: "c1" } });
const edge = (source: string, target: string, when = "") => ({ id: `${source}-${when}-${target}`, source, target, when });
const targets = (g: AutomationGraph, source: string, when = "") =>
    g.edges.filter((e) => e.source === source && (e.when ?? "") === when).map((e) => e.target);

describe("normalizeGraph", () => {
    it("adds a missing trigger and puts the trigger first", () => {
        expect(normalizeGraph(null).nodes.map((n) => n.type)).toEqual(["trigger"]);
        const g = normalizeGraph({ nodes: [tag("a"), node("trigger", "trigger")], edges: [] });
        expect(g.nodes[0].id).toBe("trigger");
    });
});

describe("nodePorts", () => {
    it("always shows a condition's two branches", () => {
        const g: AutomationGraph = { nodes: [node("trigger", "trigger"), node("c", "condition")], edges: [] };
        expect(nodePorts(g, "c").map((p) => [p.port, p.showEmpty])).toEqual([
            ["true", true],
            ["false", true],
        ]);
    });

    it("gives a switch one column per case and flags paths on removed cases", () => {
        const sw = node("s", "action", { action: "warmbly.ai_switch", config: { cases: ["yes", "no"] } });
        const g: AutomationGraph = { nodes: [node("trigger", "trigger"), sw, tag("a")], edges: [edge("s", "a", "label:gone")] };
        const ports = nodePorts(g, "s");
        expect(ports.map((p) => p.port)).toEqual(["label:yes", "label:no", "label:gone"]);
        expect(ports[2].tone).toBe("warn");
    });

    it("splits an action into Then and On error once it has an error path", () => {
        const g: AutomationGraph = { nodes: [node("trigger", "trigger"), tag("a"), tag("b")], edges: [edge("a", "b", "error")] };
        expect(nodePorts(g, "a").map((p) => [p.port, p.label])).toEqual([
            ["", "Then"],
            ["error", "On error"],
        ]);
    });
});

describe("insertNode", () => {
    const base: AutomationGraph = { nodes: [node("trigger", "trigger"), tag("a")], edges: [edge("trigger", "a")] };

    it("inserts between two steps and keeps the rest of the chain", () => {
        const g = insertNode(base, { from: "trigger", port: "", before: "a" }, tag("n"));
        expect(targets(g, "trigger")).toEqual(["n"]);
        expect(targets(g, "n")).toEqual(["a"]);
    });

    it("continues a condition inserted mid-chain on its yes branch", () => {
        const g = insertNode(base, { from: "trigger", port: "", before: "a" }, node("c", "condition"));
        expect(targets(g, "c", "true")).toEqual(["a"]);
        expect(targets(g, "c", "false")).toEqual([]);
    });

    it("appends at the end of a branch", () => {
        const g = insertNode(base, { from: "a", port: "", before: null }, tag("n"));
        expect(targets(g, "a")).toEqual(["n"]);
    });
});

describe("removalImpact", () => {
    it("reconnects around a step with one way on", () => {
        const g: AutomationGraph = {
            nodes: [node("trigger", "trigger"), tag("a"), tag("b")],
            edges: [edge("trigger", "a"), edge("a", "b")],
        };
        const { graph, dropped } = removalImpact(g, "a");
        expect(dropped).toEqual([]);
        expect(targets(graph, "trigger")).toEqual(["b"]);
    });

    it("keeps the branch label when reconnecting", () => {
        const g: AutomationGraph = {
            nodes: [node("trigger", "trigger"), node("c", "condition"), tag("a"), tag("b")],
            edges: [edge("trigger", "c"), edge("c", "a", "false"), edge("a", "b")],
        };
        expect(targets(removalImpact(g, "a").graph, "c", "false")).toEqual(["b"]);
    });

    it("drops the steps only a removed branch led to, and keeps shared ones", () => {
        const g: AutomationGraph = {
            nodes: [node("trigger", "trigger"), node("c", "condition"), tag("y"), tag("n"), tag("shared")],
            edges: [edge("trigger", "c"), edge("c", "y", "true"), edge("c", "n", "false"), edge("y", "shared"), edge("trigger", "shared")],
        };
        const { graph, dropped } = removalImpact(g, "c");
        expect(dropped.sort()).toEqual(["n", "y"]);
        expect(graph.nodes.map((n) => n.id).sort()).toEqual(["shared", "trigger"]);
        expect(graph.edges.every((e) => e.source !== "y" && e.target !== "y")).toBe(true);
    });
});

describe("gotoTargets", () => {
    it("never offers a step that would loop back", () => {
        const g: AutomationGraph = {
            nodes: [node("trigger", "trigger"), tag("a"), tag("b"), tag("c")],
            edges: [edge("trigger", "a"), edge("a", "b"), edge("trigger", "c")],
        };
        expect(gotoTargets(g, "b").map((n) => n.id)).toEqual(["c"]);
        expect(
            gotoTargets(addGoto(g, "c", "", "b"), "b")
                .map((n) => n.id)
                .includes("c"),
        ).toBe(false);
    });
});

describe("renameCases and healForSave", () => {
    const sw = (cases: string[]) => node("s", "action", { action: "warmbly.ai_switch", config: { cases } });

    it("moves a case's path with its rename", () => {
        const g: AutomationGraph = { nodes: [node("trigger", "trigger"), sw(["hot", "cold"]), tag("a")], edges: [edge("s", "a", "label:hot")] };
        expect(renameCases(g, "s", ["hot", "cold"], ["warm", "cold"]).edges[0].when).toBe("label:warm");
    });

    it("saves a path on a removed case as always", () => {
        const g: AutomationGraph = { nodes: [node("trigger", "trigger"), sw(["a", "b"]), tag("x")], edges: [edge("s", "x", "label:gone")] };
        expect(healForSave(g).edges[0].when).toBe("");
    });
});

describe("issues", () => {
    it("names what a step is missing", () => {
        expect(nodeIssue(node("a", "action"), "campaign.reply_received")).toBe("Choose what this step does.");
        expect(nodeIssue(node("a", "action", { action: "warmbly.add_tag", config: {} }), "x")).toBe("Pick a label.");
        expect(nodeIssue(tag("a"), "x")).toBeNull();
    });

    it("warns about unreachable steps without blocking", () => {
        const g: AutomationGraph = { nodes: [node("trigger", "trigger"), tag("a")], edges: [] };
        expect(graphIssues(g, "x", ["trigger", "a"])).toEqual([{ id: "a", message: "Nothing leads here, so this step never runs.", blocking: false }]);
    });
});
