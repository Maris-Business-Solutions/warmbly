// When Remie may interrupt with a tip, and which one. Tips are Advisor findings
// worth acting on, offered through Remie's speech bubble. The rules exist so a
// tip stays rare enough to be read: a tip shown too often is a tip ignored.
//
// Memory is per member and workspace, in localStorage. It records which kinds
// of problem were already offered (and at what severity, so one that gets
// worse may be offered once more), today's count, and how tips were received.

import type { AdvisorFinding, AdvisorSurface } from "@/lib/api/models/app/advisor/Advisor";
import { SEVERITY_RANK } from "@/lib/api/models/app/advisor/Advisor";

export const TIP_RULES = {
    // Only findings that are hurting sending now, or will.
    minRank: SEVERITY_RANK.high,
    perDay: 2,
    gapMs: 30 * 60_000,
    // Nothing in the first moments of a visit; people arrive to do something.
    warmupMs: 20_000,
    // The member must have paused (no keys, clicks or scrolling) this long.
    idleMs: 4_000,
    // A bubble nobody touches leaves on its own.
    autoHideMs: 15_000,
    // This many tips closed or ignored in a row pauses tips for backoffMs.
    ignoreStreak: 2,
    backoffMs: 3 * 24 * 60 * 60_000,
} as const;

export type TipMemory = {
    // Kind of problem (detector key) -> the severity rank it had when offered.
    seen: Record<string, number>;
    day: string;
    shownToday: number;
    lastShownAt: number;
    ignored: number;
    pausedUntil: number;
    off: boolean;
};

export const emptyMemory = (): TipMemory => ({
    seen: {},
    day: "",
    shownToday: 0,
    lastShownAt: 0,
    ignored: 0,
    pausedUntil: 0,
    off: false,
});

const dayOf = (now: number) => new Date(now).toISOString().slice(0, 10);

// The dashboard tab each surface's fix lives on, to prefer a tip about the page
// the member is already looking at.
const SURFACE_PATH: Record<AdvisorSurface, string> = {
    campaigns: "/app/campaigns",
    emails: "/app/emails",
    deliverability: "/app/deliverability",
    contacts: "/app/contacts",
    analytics: "/app/analytics",
    settings: "/app/settings",
};

export function onSurfacePage(f: AdvisorFinding, pathname: string): boolean {
    return pathname.startsWith(SURFACE_PATH[f.surface]);
}

// One thing Remie suggests: every open finding of one kind, so three steps
// with the same copy problem read as one thing to fix rather than three.
export type RemieSuggestion = {
    key: string;
    text: string;
    severity: AdvisorFinding["severity"];
    findings: AdvisorFinding[];
};

export function suggestionsFrom(findings: AdvisorFinding[], minRank: number): RemieSuggestion[] {
    const byKind = new Map<string, AdvisorFinding[]>();
    for (const f of findings) {
        if (f.status !== "open" || SEVERITY_RANK[f.severity] < minRank) continue;
        const list = byKind.get(f.detector_key);
        if (list) list.push(f);
        else byKind.set(f.detector_key, [f]);
    }
    // Findings arrive most severe first, so each kind's first one leads it.
    return Array.from(byKind.entries()).map(([key, list]) => {
        const lead = list[0];
        const text =
            list.length > 1 && lead.group_title
                ? lead.group_title.replace("{count}", String(list.length))
                : list.length > 1
                  ? `${lead.title}, and ${list.length - 1} more like it`
                  : lead.title;
        return { key, text, severity: lead.severity, findings: list };
    });
}

// Why no tip may show right now, or null when one may. Findings are not
// consulted here; this is only about the member's attention.
export function tipBlockedReason(m: TipMemory, now: number): string | null {
    if (m.off) return "off";
    if (m.pausedUntil > now) return "paused";
    if (m.day === dayOf(now) && m.shownToday >= TIP_RULES.perDay) return "daily-limit";
    if (now - m.lastShownAt < TIP_RULES.gapMs) return "too-soon";
    return null;
}

// pickTip chooses the suggestion to offer, or null. Suggestions arrive most
// severe first; one about the current page wins within the same severity.
export function pickTip(
    suggestions: RemieSuggestion[],
    m: TipMemory,
    now: number,
    pathname: string,
): RemieSuggestion | null {
    if (tipBlockedReason(m, now)) return null;
    const fresh = suggestions.filter((s) => {
        const rank = SEVERITY_RANK[s.severity];
        if (rank < TIP_RULES.minRank) return false;
        const offered = m.seen[s.key];
        return offered === undefined || rank > offered;
    });
    if (fresh.length === 0) return null;
    const top = SEVERITY_RANK[fresh[0].severity];
    const sameRank = fresh.filter((s) => SEVERITY_RANK[s.severity] === top);
    return sameRank.find((s) => s.findings.some((f) => onSurfacePage(f, pathname))) ?? fresh[0];
}

export function recordShown(m: TipMemory, s: RemieSuggestion, now: number): TipMemory {
    const today = dayOf(now);
    return {
        ...m,
        seen: { ...m.seen, [s.key]: SEVERITY_RANK[s.severity] },
        day: today,
        shownToday: (m.day === today ? m.shownToday : 0) + 1,
        lastShownAt: now,
    };
}

// The member acted on a tip (fixed it or chose Not now): the streak of
// ignored tips resets.
export function recordEngaged(m: TipMemory): TipMemory {
    return { ...m, ignored: 0 };
}

// The member closed a tip or let it time out. Enough of those in a row and
// Remie goes quiet for a while.
export function recordIgnored(m: TipMemory, now: number): TipMemory {
    const ignored = m.ignored + 1;
    if (ignored >= TIP_RULES.ignoreStreak) {
        return { ...m, ignored: 0, pausedUntil: now + TIP_RULES.backoffMs };
    }
    return { ...m, ignored };
}

export function pauseForWeek(m: TipMemory, now: number): TipMemory {
    return { ...m, pausedUntil: now + 7 * 24 * 60 * 60_000 };
}

// Kinds that no longer have an open finding are dropped, so the memory cannot
// grow forever and a problem that comes back later may be mentioned again.
export function pruneSeen(m: TipMemory, openKinds: Set<string>): TipMemory {
    const seen: Record<string, number> = {};
    for (const [key, rank] of Object.entries(m.seen)) if (openKinds.has(key)) seen[key] = rank;
    return { ...m, seen };
}

const keyFor = (userId: string, orgId: string) => `remie.tips.${userId}.${orgId}`;

export function loadMemory(userId: string, orgId: string): TipMemory {
    try {
        const raw = localStorage.getItem(keyFor(userId, orgId));
        return raw ? { ...emptyMemory(), ...JSON.parse(raw) } : emptyMemory();
    } catch {
        return emptyMemory();
    }
}

export function saveMemory(userId: string, orgId: string, m: TipMemory) {
    try {
        localStorage.setItem(keyFor(userId, orgId), JSON.stringify(m));
    } catch {
        // Storage full or blocked: tips just forget, which errs toward quiet.
    }
}

// What Remie is asked to do for a suggestion: each finding with the change the
// Advisor recommends, fixed through Remie's own tools and approvals.
export function fixPromptFor(s: RemieSuggestion): string {
    const lines = s.findings.map((f, i) => {
        const fix = f.action?.label ?? f.remedy;
        return `${i + 1}. ${f.title}. Recommended: ${fix}`;
    });
    return [
        s.findings.length > 1 ? "Please fix these:" : "Please fix this:",
        ...lines,
        "Check the current state first, make the smallest change that resolves each one, and ask me before changing anything.",
    ].join("\n");
}
