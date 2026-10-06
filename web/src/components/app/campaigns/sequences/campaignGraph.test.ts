import { describe, expect, it } from "vitest";
import type Sequence from "@/lib/api/models/app/campaigns/sequences/Sequence";
import type { SequenceBranch } from "@/lib/api/models/app/campaigns/sequences/Branching";
import { layoutFlow } from "@/components/app/flow/tree";
import {
    ELSE_PORT,
    ENTRY_ID,
    addCondition,
    campaignSource,
    casePort,
    conditionText,
    gotoCandidates,
    isStopId,
    moveCondition,
    normalize,
    planRemoval,
    routePort,
    stepPorts,
    emailBodyEmpty,
    DEFAULT_CONDITION,
} from "./campaignGraph";

const br = (id: string, target: string | null, conditions: SequenceBranch["conditions"] = []): SequenceBranch => ({ branch_id: id, target_step_id: target, conditions });
const opened = [{ field: "opened" as const, operator: "within_days" as const, value: 3 }];
const step = (id: string, branches: SequenceBranch[] = [], extra: Partial<Sequence> = {}): Sequence =>
    ({ id, name: "", subject: "", body_plain: "", body_html: "", body_sync: false, body_code: false, wait_after: 0, thread_reply: true, x: 0, y: 0, kind: "email", conditions: { branches }, updated_at: new Date(), created_at: new Date(), ...extra }) as Sequence;
const size = () => ({ w: 280, h: 78 });

describe("stepPorts", () => {
    it("is a plain chain when a step has only a catch-all", () => {
        expect(stepPorts(step("a", [br("x", "b")]))).toEqual([{ port: ELSE_PORT, targets: ["b"] }]);
    });

    it("draws each condition as a column, then Otherwise", () => {
        const ports = stepPorts(step("a", [br("x", "c"), br("y", "b", opened)]));
        expect(ports.map((p) => p.label)).toEqual(["Opened the email within 3d · instant", "Otherwise"]);
        expect(ports[1].targets).toEqual(["c"]);
    });

    it("draws a null target as an End terminal", () => {
        const [p] = stepPorts(step("a", [br("y", null, opened)]));
        expect(isStopId(p.targets[0])).toBe(true);
    });

    it("gives a switch one column per case and keeps the first catch-all only", () => {
        const s = step("s", [br("c1", "x", [{ field: "ai_label", operator: "is", label: "Hot" }]), br("e1", "y"), br("e2", "z")], {
            kind: "action",
            action: { type: "switch", switch_cases: ["hot", "cold"] },
        });
        const ports = stepPorts(s);
        expect(ports.map((p) => [p.port, p.targets])).toEqual([
            [casePort("hot"), ["x"]],
            [casePort("cold"), []],
            [ELSE_PORT, ["y"]],
        ]);
    });
});

describe("campaignSource", () => {
    it("starts at the entry and offers no insert before the first step", () => {
        const l = layoutFlow(campaignSource([step("a"), step("b")], size));
        expect(l.nodes[0].key).toBe(ENTRY_ID);
        expect(l.edges.find((e) => e.from === ENTRY_ID)!.insert).toBeNull();
        expect(l.nodes.find((n) => n.key === "b")!.detached).toBe(true);
    });

    it("lets an empty campaign add its first step from the entry", () => {
        const l = layoutFlow(campaignSource([], size));
        expect(l.nodes.find((n) => n.kind === "add")?.ref).toBe(ENTRY_ID);
    });
});

describe("routePort", () => {
    it("sets or creates the catch-all", () => {
        expect(routePort(step("a"), ELSE_PORT, "b").map((b) => b.target_step_id)).toEqual(["b"]);
        expect(routePort(step("a", [br("x", "b")]), ELSE_PORT, "c").map((b) => [b.branch_id, b.target_step_id])).toEqual([["x", "c"]]);
    });

    it("creates a switch case branch named after the case", () => {
        const s = step("s", [], { kind: "action", action: { type: "switch", switch_cases: ["Hot", "Cold"] } });
        const [b] = routePort(s, casePort("cold"), "t");
        expect(b.conditions).toEqual([{ field: "ai_label", operator: "is", label: "Cold" }]);
    });
});

describe("conditions", () => {
    it("adds a new condition after the existing ones and before the catch-all", () => {
        const { branches, branch } = addCondition(step("a", [br("e", "z"), br("c", "y", opened)]), DEFAULT_CONDITION);
        expect(branches.map((b) => b.branch_id)).toEqual(["c", branch.branch_id, "e"]);
        expect(branch.target_step_id).toBeNull();
    });

    it("reorders conditions and never the catch-all", () => {
        const s = step("a", [br("c1", null, opened), br("c2", null, opened), br("e", "z")]);
        expect(moveCondition(s, "c2", -1).map((b) => b.branch_id)).toEqual(["c2", "c1", "e"]);
        expect(moveCondition(s, "c1", -1).map((b) => b.branch_id)).toEqual(["c1", "c2", "e"]);
    });

    it("reads as a label", () => {
        expect(conditionText(br("x", null, [{ field: "reply_intent", operator: "is", label: "agreed" }]))).toBe("Intent: Agreed");
        expect(conditionText(br("x", null, [{ field: "random", operator: "chance", value: 30 }]))).toBe("30% random");
    });

    it("keeps only the first catch-all", () => {
        expect(normalize([br("e1", "a"), br("c", "b", opened), br("e2", "c")]).map((b) => b.branch_id)).toEqual(["c", "e1"]);
    });
});

describe("planRemoval", () => {
    it("reconnects around a step with one way on", () => {
        const steps = [step("a", [br("x", "b", opened)]), step("b", [br("y", "c")]), step("c")];
        const plan = planRemoval(steps, "b");
        expect(plan.successor).toBe("c");
        expect(plan.rewrites).toEqual([{ id: "a", branches: [br("x", "c", opened)] }]);
        expect(plan.stranded).toEqual([]);
    });

    it("ends a conditional path and drops a plain one when there is no single way on", () => {
        const steps = [step("a", [br("x", "b", opened), br("e", "b")]), step("b", [br("c1", "c", opened), br("e2", "d")]), step("c"), step("d")];
        const plan = planRemoval(steps, "b");
        expect(plan.rewrites[0].branches).toEqual([br("x", null, opened)]);
        expect(plan.stranded.sort()).toEqual(["c", "d"]);
    });
});

describe("gotoCandidates", () => {
    it("never offers a step that leads back", () => {
        const steps = [step("a", [br("x", "b")]), step("b"), step("c")];
        expect(gotoCandidates(steps, "b").map((s) => s.id)).toEqual(["c"]);
    });
});

describe("emailBodyEmpty", () => {
    it("treats tag-only markup as empty", () => {
        expect(emailBodyEmpty("<div></div>")).toBe(true);
        expect(emailBodyEmpty("<p>Hi</p>")).toBe(false);
        expect(emailBodyEmpty('<img src="x">')).toBe(false);
    });
});
