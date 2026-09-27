import { describe, expect, it } from "vitest";
import reviveDates from "./reviveDates";
import shareDeep from "./shareDeep";

const wire = () => reviveDates({ id: "c1", name: "Q4", updated_at: "2026-09-27T11:53:26Z", tags: ["a"], range: { from: "2026-09-01T00:00:00Z" } });

describe("shareDeep", () => {
    it("keeps the previous object when a refetch returns the same data", () => {
        const prev = wire();
        expect(shareDeep(prev, wire())).toBe(prev);
    });

    it("gives a changed value a new object and keeps the unchanged branches", () => {
        const prev = wire();
        const next = { ...wire(), name: "Q1" };
        const shared = shareDeep(prev, next) as typeof prev;
        expect(shared).not.toBe(prev);
        expect(shared.name).toBe("Q1");
        expect(shared.updated_at).toBe(prev.updated_at);
        expect(shared.tags).toBe(prev.tags);
        expect(shared.range).toBe(prev.range);
    });

    it("hands out the new Date when the time moved", () => {
        const prev = wire();
        const next = reviveDates({ ...prev, updated_at: "2026-09-27T12:00:00Z" });
        const shared = shareDeep(prev, next) as typeof prev;
        expect(shared).not.toBe(prev);
        expect(shared.updated_at).toBe(next.updated_at);
    });

    it("notices a key that was added or removed", () => {
        expect(shareDeep({ a: 1 }, { a: 1, b: undefined })).toEqual({ a: 1, b: undefined });
        expect(shareDeep({ a: 1, b: 2 }, { a: 1 })).toEqual({ a: 1 });
        expect(shareDeep([1, 2], [1])).toEqual([1]);
    });
});
