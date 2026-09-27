import { afterEach, describe, expect, it, vi } from "vitest";
import { formatWindow, loadCampaignPeriod, periodWindow, savePreset } from "./campaignPeriod";

describe("periodWindow", () => {
    afterEach(() => vi.useRealTimers());

    it("counts a preset in UTC days, today included", () => {
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-09-27T23:30:00Z"));
        expect(periodWindow({ key: "7d" })).toEqual({ from: "2026-09-21", to: "2026-09-27" });
        expect(periodWindow({ key: "30d" })).toEqual({ from: "2026-08-29", to: "2026-09-27" });
    });

    it("ends a preset on the day it is given", () => {
        expect(periodWindow({ key: "7d" }, "2026-03-02")).toEqual({ from: "2026-02-24", to: "2026-03-02" });
    });

    it("sends no window for all time and the chosen days for custom", () => {
        expect(periodWindow({ key: "all" })).toBeNull();
        expect(periodWindow({ key: "custom", from: "2026-09-01", to: "2026-09-07" })).toEqual({ from: "2026-09-01", to: "2026-09-07" });
    });
});

describe("formatWindow", () => {
    it("names the year once when both ends share it", () => {
        expect(formatWindow({ from: "2026-09-01", to: "2026-09-27" })).toBe("Sep 1 – Sep 27, 2026");
    });
    it("names both years across a new year", () => {
        expect(formatWindow({ from: "2025-12-20", to: "2026-01-03" })).toBe("Dec 20, 2025 – Jan 3, 2026");
    });
    it("names one day once", () => {
        expect(formatWindow({ from: "2026-09-07", to: "2026-09-07" })).toBe("Sep 7, 2026");
    });
});

describe("loadCampaignPeriod", () => {
    it("defaults to all time and remembers the last preset", () => {
        const values = new Map<string, string>();
        vi.mocked(localStorage.getItem).mockImplementation((key: string) => values.get(key) ?? null);
        vi.mocked(localStorage.setItem).mockImplementation((key: string, value: string) => {
            values.set(key, value);
        });
        expect(loadCampaignPeriod()).toEqual({ key: "all" });
        savePreset("30d");
        expect(loadCampaignPeriod()).toEqual({ key: "30d" });
        localStorage.setItem("warmbly.campaign-period", "custom");
        expect(loadCampaignPeriod()).toEqual({ key: "all" });
    });

    it("falls back to all time when storage is blocked", () => {
        vi.mocked(localStorage.getItem).mockImplementation(() => {
            throw new Error("blocked");
        });
        expect(loadCampaignPeriod()).toEqual({ key: "all" });
    });
});
