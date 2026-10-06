// Pure operations on an automation graph for the tree builder: how each node's
// outgoing edges read as ports, inserting and removing steps without leaving
// dangling edges, and the per-step setup problems the canvas flags.

import type { AutomationCondition, AutomationEdge, AutomationGraph, AutomationNode } from "@/lib/api/models/app/automations/Automation";
import {
    actionNeedsChannel,
    actionNeedsURL,
    isNativeAction,
    nativeActionNeeds,
    triggerCarriesThread,
} from "@/lib/api/models/app/automations/meta";
import type { FlowPort, FlowSource, InsertPoint } from "@/components/app/flow/tree";
import { reachableFrom } from "@/components/app/flow/tree";

export const TRIGGER_ID = "trigger";
export const SWITCH_ACTION = "warmbly.ai_switch";
const CASE_PREFIX = "label:";

export const caseWhen = (name: string) => CASE_PREFIX + name.trim();

export function newId(): string {
    try {
        return crypto.randomUUID();
    } catch {
        return `n_${Math.floor(performance.now())}_${Math.random().toString(36).slice(2, 8)}`;
    }
}

export function isSwitch(n?: AutomationNode): boolean {
    return n?.type === "action" && n.action === SWITCH_ACTION;
}

export function switchCases(n?: AutomationNode): string[] {
    const raw = n?.config?.cases;
    return Array.isArray(raw) ? raw.map((c) => String(c).trim()).filter(Boolean) : [];
}

// A graph always has exactly one trigger node, first.
export function normalizeGraph(g?: AutomationGraph | null): AutomationGraph {
    const nodes = g?.nodes?.length ? [...g.nodes] : [];
    const edges = (g?.edges ?? []).map((e) => ({ ...e, when: e.when ?? "" }));
    const t = nodes.findIndex((n) => n.type === "trigger");
    if (t < 0) nodes.unshift({ id: TRIGGER_ID, type: "trigger", x: 0, y: 0 });
    else if (t > 0) nodes.unshift(nodes.splice(t, 1)[0]);
    return { nodes, edges };
}

export function triggerId(g: AutomationGraph): string {
    return g.nodes.find((n) => n.type === "trigger")?.id ?? TRIGGER_ID;
}

const targetsOf = (g: AutomationGraph, id: string, when: string) =>
    g.edges.filter((e) => e.source === id && (e.when ?? "") === when).map((e) => e.target);

// How a node's outgoing edges are drawn. The order is the order of the columns.
export function nodePorts(g: AutomationGraph, id: string): FlowPort[] {
    const n = g.nodes.find((x) => x.id === id);
    if (!n || n.type === "stop") return [];
    if (n.type === "trigger") return [{ port: "", targets: targetsOf(g, id, "") }];
    if (n.type === "condition") {
        return [
            { port: "true", label: "Yes", tone: "yes", showEmpty: true, targets: targetsOf(g, id, "true") },
            { port: "false", label: "No", tone: "no", showEmpty: true, targets: targetsOf(g, id, "false") },
        ];
    }
    const ports: FlowPort[] = [];
    const errors = targetsOf(g, id, "error");
    if (isSwitch(n)) {
        const cases = switchCases(n);
        const known = new Set(cases.map((c) => c.toLowerCase()));
        for (const c of cases) ports.push({ port: caseWhen(c), label: c, tone: "case", showEmpty: true, targets: targetsOf(g, id, caseWhen(c)) });
        // Case paths whose case was renamed away; the save turns them into always.
        const stale = new Set(
            g.edges
                .filter((e) => e.source === id && (e.when ?? "").startsWith(CASE_PREFIX))
                .map((e) => e.when!)
                .filter((w) => !known.has(w.slice(CASE_PREFIX.length).trim().toLowerCase())),
        );
        for (const w of stale) ports.push({ port: w, label: `${w.slice(CASE_PREFIX.length)} (removed)`, tone: "warn", targets: targetsOf(g, id, w) });
        const always = targetsOf(g, id, "");
        if (always.length || cases.length === 0) ports.push({ port: "", label: "Always", tone: "neutral", targets: always, showEmpty: cases.length > 0 });
    } else {
        const next = targetsOf(g, id, "");
        if (errors.length) ports.push({ port: "", label: "Then", tone: "neutral", showEmpty: true, targets: next });
        else ports.push({ port: "", targets: next });
    }
    if (errors.length) ports.push({ port: "error", label: "On error", tone: "error", targets: errors });
    return ports;
}

export function flowSource(g: AutomationGraph, size: FlowSource["size"]): FlowSource {
    return {
        roots: [triggerId(g)],
        nodeIds: g.nodes.map((n) => n.id),
        ports: (id) => nodePorts(g, id),
        size,
        appendPort: (id) => (g.nodes.find((n) => n.id === id)?.type === "stop" ? null : ""),
    };
}

// The port a node inserted in front of an existing step continues on, so the
// steps after it keep running exactly as before.
export function continuePort(n: AutomationNode): string | null {
    if (n.type === "stop") return null;
    if (n.type === "condition") return "true";
    return "";
}

const dedupe = (edges: AutomationEdge[]) => {
    const seen = new Set<string>();
    return edges.filter((e) => {
        const k = `${e.source}|${e.when ?? ""}|${e.target}`;
        if (seen.has(k) || e.source === e.target) return false;
        seen.add(k);
        return true;
    });
};

export function insertNode(g: AutomationGraph, at: InsertPoint, node: AutomationNode): AutomationGraph {
    const edges = [...g.edges];
    if (at.before) {
        const i = edges.findIndex((e) => e.source === at.from && (e.when ?? "") === at.port && e.target === at.before);
        const port = continuePort(node);
        if (i >= 0 && port !== null) {
            edges[i] = { ...edges[i], target: node.id };
            edges.push({ id: newId(), source: node.id, target: at.before, when: port });
            return { nodes: [...g.nodes, node], edges: dedupe(edges) };
        }
    }
    edges.push({ id: newId(), source: at.from, target: node.id, when: at.port });
    return { nodes: [...g.nodes, node], edges: dedupe(edges) };
}

export function addGoto(g: AutomationGraph, from: string, port: string, target: string): AutomationGraph {
    return { nodes: g.nodes, edges: dedupe([...g.edges, { id: newId(), source: from, target, when: port }]) };
}

export function removeEdge(g: AutomationGraph, from: string, port: string, target: string): AutomationGraph {
    return { nodes: g.nodes, edges: g.edges.filter((e) => !(e.source === from && (e.when ?? "") === port && e.target === target)) };
}

// Steps a go-to from `from` may jump to: anything that would not loop back.
export function gotoTargets(g: AutomationGraph, from: string): AutomationNode[] {
    const ports = (id: string) => nodePorts(g, id);
    return g.nodes.filter((n) => n.type !== "trigger" && n.id !== from && !reachableFrom({ ports }, n.id).has(from));
}

function reachableSet(g: AutomationGraph): Set<string> {
    return reachableFrom({ ports: (id) => nodePorts(g, id) }, triggerId(g));
}

// What removing a step costs: the steps that only it leads to, which go with it.
export function removalImpact(g: AutomationGraph, id: string): { graph: AutomationGraph; dropped: string[] } {
    const outTargets = [...new Set(g.edges.filter((e) => e.source === id).map((e) => e.target))];
    let edges = g.edges.filter((e) => e.source !== id);
    if (outTargets.length <= 1) {
        // One way on: whatever led here now leads there.
        const next = outTargets[0];
        edges = edges.flatMap((e) => (e.target !== id ? [e] : next ? [{ ...e, target: next }] : []));
        return { graph: { nodes: g.nodes.filter((n) => n.id !== id), edges: dedupe(edges) }, dropped: [] };
    }
    const before = reachableSet(g);
    edges = edges.filter((e) => e.target !== id);
    let graph: AutomationGraph = { nodes: g.nodes.filter((n) => n.id !== id), edges };
    const after = reachableSet(graph);
    const dropped = graph.nodes.filter((n) => before.has(n.id) && !after.has(n.id)).map((n) => n.id);
    const gone = new Set(dropped);
    graph = {
        nodes: graph.nodes.filter((n) => !gone.has(n.id)),
        edges: graph.edges.filter((e) => !gone.has(e.source) && !gone.has(e.target)),
    };
    return { graph, dropped };
}

// A switch's case paths follow their case through a rename (matched by
// position), so editing a case name never strands the steps under it.
export function renameCases(g: AutomationGraph, id: string, prev: string[], next: string[]): AutomationGraph {
    if (prev.length !== next.length) return g;
    const map = new Map<string, string>();
    prev.forEach((p, i) => {
        if (p !== next[i] && next[i]) map.set(caseWhen(p), caseWhen(next[i]));
    });
    if (!map.size) return g;
    return {
        nodes: g.nodes,
        edges: g.edges.map((e) => (e.source === id && map.has(e.when ?? "") ? { ...e, when: map.get(e.when!)! } : e)),
    };
}

// The graph exactly as it is saved: case paths whose case is gone run always.
export function healForSave(g: AutomationGraph): AutomationGraph {
    return {
        nodes: g.nodes,
        edges: g.edges.map((e) => {
            const w = e.when ?? "";
            if (!w.startsWith(CASE_PREFIX)) return { ...e, when: w };
            const src = g.nodes.find((n) => n.id === e.source);
            const known = isSwitch(src) ? switchCases(src).map((c) => c.toLowerCase()) : [];
            return known.includes(w.slice(CASE_PREFIX.length).trim().toLowerCase()) ? { ...e, when: w } : { ...e, when: "" };
        }),
    };
}

// The first thing stopping a step from running, worded for the person fixing it.
export function nodeIssue(n: AutomationNode, trigger: string): string | null {
    if (n.type === "condition") {
        const c: AutomationCondition | undefined = n.condition;
        if (c?.field === "ai" && !String(c.prompt ?? "").trim()) return "Write the question AI should answer.";
        if (c?.field === "expression" && !String(c.expression ?? "").trim()) return "Write the expression to test.";
        return null;
    }
    if (n.type !== "action") return null;
    const action = String(n.action ?? "");
    const cfg = n.config ?? {};
    const str = (k: string) => String(cfg[k] ?? "").trim();
    const list = (k: string) => (Array.isArray(cfg[k]) ? (cfg[k] as unknown[]).map((v) => String(v).trim()).filter(Boolean) : []);
    if (!action) return "Choose what this step does.";
    if (isNativeAction(action)) {
        const need = nativeActionNeeds(action);
        if (need === "tag" && !str("category_id")) return "Pick a label.";
        if (need === "label" && list("label_ids").length === 0) return "Pick at least one label.";
        if (need === "label" && !triggerCarriesThread(trigger)) return "Labelling a conversation only works on a “Reply received” trigger.";
        if (need === "deal" && (!str("deal_pipeline_id") || !str("deal_stage_id"))) return "Pick a pipeline and a stage.";
        if (need === "automation" && !str("automation_id")) return "Pick the automation to run.";
        if (need === "vars" && !(Array.isArray(cfg.set_vars) && (cfg.set_vars as { key?: string }[]).some((v) => String(v?.key ?? "").trim())))
            return "Name at least one variable.";
        if (need === "event" && !str("event_name")) return "Name the event to fire.";
        if (need === "contact" && !str("email")) return "Set the contact's email.";
        if (need === "campaign" && !str("campaign_id")) return "Pick a campaign.";
        const valueSwitch = need === "ai_switch" && String(cfg.switch_on ?? "ai") === "value";
        if ((need === "ai_step" || need === "ai_switch") && !valueSwitch && !str("instruction")) return "Write the instruction for AI.";
        const mode = String(cfg.mode ?? "agent");
        if (need === "ai_step" && mode === "agent" && list("allowed_actions").length === 0) return "Allow the agent at least one action.";
        if (need === "ai_step" && mode === "classify" && list("labels").length < 2) return "Add at least two labels to classify into.";
        if (need === "ai_step" && mode === "extract" && list("output_keys").length === 0) return "Add at least one value to extract.";
        if (need === "ai_switch" && list("cases").length < 2) return "Add at least two cases.";
        if (valueSwitch && !str("switch_value")) return "Set the value to match.";
        return null;
    }
    if (!n.connection_id) return "Choose which integration runs this.";
    if (actionNeedsChannel(action) && !str("channel")) return "Set the channel to post in.";
    if (actionNeedsURL(action) && !str("url")) return "Set the webhook URL.";
    return null;
}

export interface GraphIssue {
    id: string;
    message: string;
    blocking: boolean;
}

// Every problem on the canvas, in tree order. Blocking ones stop a save; the
// rest are warnings (a step nothing leads to never runs, but saving is fine).
export function graphIssues(g: AutomationGraph, trigger: string, order: string[]): GraphIssue[] {
    const reach = reachableSet(g);
    const out: GraphIssue[] = [];
    const rank = new Map(order.map((id, i) => [id, i]));
    const nodes = [...g.nodes].sort((a, b) => (rank.get(a.id) ?? 1e9) - (rank.get(b.id) ?? 1e9));
    for (const n of nodes) {
        const m = nodeIssue(n, trigger);
        if (m) out.push({ id: n.id, message: m, blocking: true });
        else if (!reach.has(n.id)) out.push({ id: n.id, message: "Nothing leads here, so this step never runs.", blocking: false });
    }
    return out;
}
