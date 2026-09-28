// Narrows a test on the workspace's own panel to the seed inboxes someone
// wants to open and check by hand. Nothing chosen means the usual pick.

import React from "react";
import { Checkbox } from "@/components/ui/checkbox";
import type { PlacementWorkspaceSeed } from "@/lib/api/models/app/placement/Placement";
import { cn } from "@/lib/utils";
import { seedBlocker } from "./placementTests";

export default function SeedChooser({
    seeds,
    senderEmail,
    perTest,
    value,
    onChange,
}: {
    seeds: PlacementWorkspaceSeed[];
    senderEmail?: string;
    perTest: number;
    value: string[];
    onChange: (ids: string[]) => void;
}) {
    const rows = React.useMemo(
        () => [...seeds].sort((a, b) => a.label.localeCompare(b.label) || a.email.localeCompare(b.email)),
        [seeds],
    );
    const usable = React.useMemo(() => rows.filter((s) => !seedBlocker(s, senderEmail)), [rows, senderEmail]);
    const chosen = new Set(value);
    const chosenUsable = usable.filter((s) => chosen.has(s.email_account_id)).length;

    const families = React.useMemo(() => {
        const out: { family: string; label: string; ids: string[] }[] = [];
        for (const s of usable) {
            let f = out.find((x) => x.family === s.family);
            if (!f) {
                f = { family: s.family, label: s.label, ids: [] };
                out.push(f);
            }
            f.ids.push(s.email_account_id);
        }
        return out;
    }, [usable]);

    const toggle = (id: string) =>
        onChange(chosen.has(id) ? value.filter((v) => v !== id) : [...value, id]);
    const toggleFamily = (ids: string[]) => {
        const all = ids.every((id) => chosen.has(id));
        onChange(all ? value.filter((v) => !ids.includes(v)) : [...value, ...ids.filter((id) => !chosen.has(id))]);
    };

    return (
        <div className="mt-2 rounded-md border border-slate-200 overflow-hidden">
            <div className="px-3 h-8 flex items-center gap-2 border-b border-slate-100 bg-slate-50/60">
                <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Send to</span>
                <span className="min-w-0 truncate text-[11px] text-slate-500">
                    {chosenUsable === 0
                        ? `Any ${Math.min(perTest || usable.length, usable.length)}, spread across providers`
                        : `${chosenUsable} chosen, each gets a copy`}
                </span>
                {value.length > 0 && (
                    <button
                        type="button"
                        onClick={() => onChange([])}
                        className="ml-auto h-5 px-1.5 rounded text-[11px] text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-colors"
                    >
                        Clear
                    </button>
                )}
            </div>
            {families.length > 1 && (
                <div className="px-3 py-1.5 flex flex-wrap gap-1 border-b border-slate-100">
                    {families.map((f) => {
                        const all = f.ids.every((id) => chosen.has(id));
                        return (
                            <button
                                key={f.family}
                                type="button"
                                aria-pressed={all}
                                onClick={() => toggleFamily(f.ids)}
                                className={cn(
                                    "h-5 px-1.5 rounded-md border text-[10.5px] font-medium inline-flex items-center gap-1 transition-colors",
                                    all
                                        ? "border-sky-200 bg-sky-50 text-sky-700"
                                        : "border-slate-200 bg-white text-slate-600 hover:border-slate-300",
                                )}
                            >
                                {f.label}
                                <span className="font-mono tabular-nums text-slate-400">{f.ids.length}</span>
                            </button>
                        );
                    })}
                </div>
            )}
            <div className="max-h-44 overflow-y-auto divide-y divide-slate-100">
                {rows.map((s) => {
                    const blocker = seedBlocker(s, senderEmail);
                    return (
                        <label
                            key={s.email_account_id}
                            className={cn(
                                "h-8 px-3 flex items-center gap-2.5",
                                blocker ? "opacity-60 cursor-not-allowed" : "cursor-pointer hover:bg-slate-50",
                            )}
                        >
                            <Checkbox
                                tone="slate"
                                checked={!blocker && chosen.has(s.email_account_id)}
                                disabled={!!blocker}
                                onChange={() => toggle(s.email_account_id)}
                            />
                            <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-slate-700">{s.email}</span>
                            <span className="shrink-0 text-[10.5px] text-slate-400">{blocker ?? s.label}</span>
                        </label>
                    );
                })}
            </div>
        </div>
    );
}
