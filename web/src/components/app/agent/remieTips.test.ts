import { describe, expect, it } from "vitest";
import type { AdvisorFinding } from "@/lib/api/models/app/advisor/Advisor";
import { SEVERITY_RANK } from "@/lib/api/models/app/advisor/Advisor";
import {
    TIP_RULES,
    emptyMemory,
    fixPromptFor,
    pickTip,
    pruneSeen,
    recordEngaged,
    recordIgnored,
    recordShown,
    suggestionsFrom,
    tipBlockedReason,
} from "./remieTips";

const NOW = Date.UTC(2026, 9, 4, 12, 0, 0);

function finding(
    id: string,
    severity: AdvisorFinding["severity"],
    kind = id,
    surface: AdvisorFinding["surface"] = "emails",
    extra: Partial<AdvisorFinding> = {},
): AdvisorFinding {
    return {
        id,
        organization_id: "org",
        detector_key: kind,
        category: "mailbox",
        severity,
        surface,
        status: "open",
        impact: 1,
        title: `t-${id}`,
        detail: "d",
        remedy: `r-${id}`,
        narrated: false,
        first_seen_at: new Date(NOW),
        last_seen_at: new Date(NOW),
        ...extra,
    };
}

const all = (fs: AdvisorFinding[]) => suggestionsFrom(fs, SEVERITY_RANK.low);

describe("suggestionsFrom", () => {
    it("folds one kind of problem into one suggestion named by its group title", () => {
        const group = { group_title: "{count} new mailboxes are sending at full volume" };
        const fs = [
            finding("a", "high", "ramp", "emails", group),
            finding("b", "high", "ramp", "emails", group),
            finding("c", "medium", "cap"),
        ];
        const got = all(fs);
        expect(got.map((s) => s.text)).toEqual(["2 new mailboxes are sending at full volume", "t-c"]);
        expect(got[0].findings).toHaveLength(2);
    });

    it("keeps findings without a group title as their own lines", () => {
        expect(all([finding("a", "high", "k"), finding("b", "high", "k")]).map((s) => s.text)).toEqual(["t-a", "t-b"]);
    });

    it("drops what is not open or below the floor", () => {
        const fs = [finding("a", "low"), { ...finding("b", "critical"), status: "applied" as const }];
        expect(suggestionsFrom(fs, SEVERITY_RANK.medium)).toEqual([]);
    });

    it("asks Remie to fix every finding with the recommended change", () => {
        const group = { group_title: "{count} things" };
        const fs = [
            finding("a", "high", "k", "emails", {
                ...group,
                action: { tool: "x", args: {}, label: "Start at 20/day instead" },
            }),
            finding("b", "high", "k", "emails", group),
        ];
        const prompt = fixPromptFor(all(fs)[0]);
        expect(prompt).toContain("1. t-a. Recommended: Start at 20/day instead");
        expect(prompt).toContain("2. t-b. Recommended: r-b");
        expect(prompt).toContain("ask me before changing anything");
    });
});

describe("pickTip", () => {
    it("never offers polish", () => {
        expect(pickTip(all([finding("a", "medium"), finding("b", "low")]), emptyMemory(), NOW, "/app")).toBeNull();
    });

    it("offers the most severe kind", () => {
        expect(pickTip(all([finding("a", "critical"), finding("b", "high")]), emptyMemory(), NOW, "/app")?.key).toBe("a");
    });

    it("prefers the current page within the same severity only", () => {
        const fs = [
            finding("a", "critical", "a", "campaigns"),
            finding("c", "critical", "c", "emails"),
            finding("b", "high", "b", "emails"),
        ];
        expect(pickTip(all(fs), emptyMemory(), NOW, "/app/emails")?.key).toBe("c");
    });

    it("offers a suggestion once, and again only when it gets worse", () => {
        const before = NOW - TIP_RULES.gapMs - 1;
        const m = recordShown(emptyMemory(), all([finding("a", "high")])[0], before);
        expect(pickTip(all([finding("a", "high")]), m, NOW, "/app")).toBeNull();
        expect(pickTip(all([finding("a", "critical")]), m, NOW, "/app")?.key).toBe("a");
    });
});

describe("attention limits", () => {
    it("keeps a gap between tips", () => {
        const m = recordShown(emptyMemory(), all([finding("a", "high")])[0], NOW - 60_000);
        expect(tipBlockedReason(m, NOW)).toBe("too-soon");
    });

    it("stops at the daily limit and resets the next day", () => {
        let m = emptyMemory();
        for (let i = 0; i < TIP_RULES.perDay; i++) {
            m = recordShown(m, all([finding(`x${i}`, "high")])[0], NOW - (TIP_RULES.perDay - i) * (TIP_RULES.gapMs + 1));
        }
        expect(tipBlockedReason(m, NOW)).toBe("daily-limit");
        expect(tipBlockedReason(m, NOW + 24 * 60 * 60_000)).toBeNull();
    });

    it("backs off after tips are ignored in a row, and engagement resets the streak", () => {
        let m = recordIgnored(emptyMemory(), NOW);
        m = recordEngaged(m);
        m = recordIgnored(m, NOW);
        expect(m.pausedUntil).toBe(0);
        m = recordIgnored(m, NOW);
        expect(tipBlockedReason(m, NOW + 60_000)).toBe("paused");
        expect(tipBlockedReason(m, NOW + TIP_RULES.backoffMs + 1)).toBeNull();
    });

    it("forgets kinds that closed", () => {
        const m = recordShown(emptyMemory(), all([finding("a", "high")])[0], NOW);
        expect(pruneSeen(m, new Set()).seen).toEqual({});
    });
});
