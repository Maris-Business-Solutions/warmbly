import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { ResourceUsage } from "./ResourceUsage";
import { resourceCSV } from "./format";

describe("fleet resource readings", () => {
    it("separates the process's resident memory from the resource budget", () => {
        const usage = { resident_mb: 137, memory_mb: 220, memory_used_mb: 256, memory_limit_mb: 1024, memory_scope: "container" as const };
        const memory = renderToStaticMarkup(<ResourceUsage usage={usage} kind="memory" live />);
        expect(memory).toContain("25.0%");
        expect(memory).toContain("container");
        const process = renderToStaticMarkup(<ResourceUsage usage={usage} kind="resident" live />);
        expect(process).toContain("137 MiB");
        expect(process).not.toContain("220 MiB");
    });

    it("does not invent percentages when telemetry is missing", () => {
        for (const kind of ["cpu", "memory", "resident"] as const) {
            expect(renderToStaticMarkup(<ResourceUsage usage={{ memory_mb: 137 }} kind={kind} live />)).toContain("Unavailable");
        }
        expect(resourceCSV({}, "cpu", true)).toBe("");
        expect(resourceCSV({}, "memory", true)).toBe("");
    });

    it("accepts measured zero CPU and marks offline snapshots stale", () => {
        const usage = { cpu_percent: 0, cpu_scope: "host" as const };
        expect(renderToStaticMarkup(<ResourceUsage usage={usage} kind="cpu" live />)).toContain("0.0%");
        expect(renderToStaticMarkup(<ResourceUsage usage={usage} kind="cpu" live={false} />)).toContain("Stale");
        expect(resourceCSV(usage, "cpu", false)).toBe("stale");
    });
});
