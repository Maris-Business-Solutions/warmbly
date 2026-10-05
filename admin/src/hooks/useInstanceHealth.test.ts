import { describe, expect, it } from "vitest";
import { findingCount, worstSeverity } from "./useInstanceHealth";
import type { CheckSeverity, InstanceHealthResult } from "@/lib/api/client/admin/instance";

function result(...severities: CheckSeverity[]): InstanceHealthResult {
    return {
        checks: severities.map((severity, i) => ({ id: String(i), severity, title: "Finding", message: "Context" })),
        summary: {
            error: severities.filter((s) => s === "error").length,
            warning: severities.filter((s) => s === "warning").length,
            info: severities.filter((s) => s === "info").length,
        },
    };
}

describe("health problem badge", () => {
    it("does not treat informational notes as problems", () => {
        const data = result("info", "info", "info");
        expect(findingCount(data)).toBe(0);
        expect(worstSeverity(data)).toBeNull();
    });

    it("counts only errors and warnings and uses the most severe problem", () => {
        expect(findingCount(result("info", "warning", "error", "info"))).toBe(2);
        expect(worstSeverity(result("info", "warning", "error"))).toBe("error");
        expect(worstSeverity(result("info", "warning"))).toBe("warning");
    });

    it("handles missing and empty results", () => {
        expect(findingCount(undefined)).toBe(0);
        expect(worstSeverity(undefined)).toBeNull();
        expect(findingCount(result())).toBe(0);
        expect(worstSeverity(result())).toBeNull();
    });
});
