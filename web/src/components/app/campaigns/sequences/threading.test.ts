import { describe, expect, it } from "vitest";
import type Sequence from "@/lib/api/models/app/campaigns/sequences/Sequence";
import type ABVariant from "@/lib/api/models/app/campaigns/ABVariant";
import { conversationOpenerFor, conversationSubjectFor, openerArmSubjects } from "./threading";

const step = (id: string, subject: string, thread_reply = true, kind: Sequence["kind"] = "email") =>
    ({ id, subject, thread_reply, kind }) as Sequence;

const variant = (step_id: string | null, name: string, subject: string, extra: Partial<ABVariant> = {}) =>
    ({ id: name, step_id, name, subject, is_active: true, is_control: false, ...extra }) as ABVariant;

describe("conversationOpenerFor", () => {
    const steps = [step("a", "Opener", false), step("w", "", true, "wait"), step("b", ""), step("c", "Fresh", false), step("d", "")];

    it("walks back past replies and control nodes to the step that opened the thread", () => {
        expect(conversationOpenerFor(steps, 2)?.id).toBe("a");
        expect(conversationOpenerFor(steps, 4)?.id).toBe("c");
        expect(conversationSubjectFor(steps, 2)).toBe("Opener");
    });

    it("has no opener for the first email", () => {
        expect(conversationOpenerFor(steps, 0)).toBeNull();
        expect(conversationSubjectFor(steps, 0)).toBeNull();
    });
});

describe("openerArmSubjects", () => {
    const opener = step("a", "Opener", false);

    it("lists the subjects the opener's own variants send instead of its own", () => {
        const arms = openerArmSubjects(opener, [
            variant("a", "Variant B", "Other angle"),
            variant("a", "Variant C", ""),
            variant("a", "Variant D", "Opener"),
            variant("a", "Paused", "Paused angle", { is_active: false }),
            variant("a", "Original", "Ignored", { is_control: true }),
            variant("b", "Elsewhere", "Another step"),
            variant(null, "Campaign-wide", "Shadowed by the step's own"),
        ]);
        expect(arms).toEqual([{ name: "Variant B", subject: "Other angle" }]);
    });

    it("lets the opener's own control row shadow campaign-wide variants", () => {
        const arms = openerArmSubjects(opener, [
            variant("a", "Original", "", { is_control: true }),
            variant(null, "Variant B", "Campaign angle"),
        ]);
        expect(arms).toEqual([]);
    });

    it("falls back to campaign-wide variants when the opener has none", () => {
        expect(openerArmSubjects(opener, [variant(null, "Variant B", "Campaign angle")])).toEqual([
            { name: "Variant B", subject: "Campaign angle" },
        ]);
    });

    it("is empty without an opener", () => {
        expect(openerArmSubjects(null, [variant("a", "Variant B", "Other angle")])).toEqual([]);
    });
});
