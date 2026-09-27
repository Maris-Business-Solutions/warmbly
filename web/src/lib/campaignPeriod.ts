// A reporting window of whole UTC days, both ends included, as "yyyy-MM-dd".
export interface DayWindow {
    from: string;
    to: string;
}

export type CampaignPeriodPreset = "7d" | "30d" | "90d" | "all";
export type CampaignPeriod = { key: CampaignPeriodPreset } | { key: "custom"; from: string; to: string };

export const PRESETS: { key: CampaignPeriodPreset; label: string; days?: number }[] = [
    { key: "7d", label: "7d", days: 7 },
    { key: "30d", label: "30d", days: 30 },
    { key: "90d", label: "90d", days: 90 },
    { key: "all", label: "All time" },
];

export const COHORT_TIP =
    "Counts the emails sent on these days (UTC), with every open, click, reply and bounce they earned, even after the period ended";

const STORAGE_KEY = "warmbly.campaign-period";

function utcDay(d: Date): string {
    return d.toISOString().slice(0, 10);
}

// The window a period asks the API for; null is all time, which sends none.
export function periodWindow(p: CampaignPeriod): DayWindow | null {
    if (p.key === "custom") return { from: p.from, to: p.to };
    const days = PRESETS.find((x) => x.key === p.key)?.days;
    if (!days) return null;
    const to = new Date();
    const from = new Date();
    from.setUTCDate(from.getUTCDate() - (days - 1));
    return { from: utcDay(from), to: utcDay(to) };
}

// "Sep 1 – Sep 27, 2026", with the year on both ends when they differ.
export function formatWindow(w: DayWindow): string {
    const f = new Date(`${w.from}T00:00:00Z`);
    const t = new Date(`${w.to}T00:00:00Z`);
    const day = (d: Date, year: boolean) =>
        d.toLocaleDateString("en-US", { month: "short", day: "numeric", year: year ? "numeric" : undefined, timeZone: "UTC" });
    if (w.from === w.to) return day(t, true);
    const sameYear = f.getUTCFullYear() === t.getUTCFullYear();
    return `${day(f, !sameYear)} – ${day(t, true)}`;
}

// The last preset chosen, shared across campaigns; a custom range is not kept.
export function loadCampaignPeriod(): CampaignPeriod {
    try {
        const v = localStorage.getItem(STORAGE_KEY);
        if (PRESETS.some((p) => p.key === v)) return { key: v as CampaignPeriodPreset };
    } catch {
        // Unavailable storage falls back to all time.
    }
    return { key: "all" };
}

export function savePreset(key: CampaignPeriodPreset) {
    try {
        localStorage.setItem(STORAGE_KEY, key);
    } catch {
        // Not persisting is fine.
    }
}
