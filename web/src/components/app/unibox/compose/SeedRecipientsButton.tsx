// Adds the workspace's own seed inboxes to a compose field, for a quick
// manual look at where a message lands. Remembers the last pick.

import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckSquare } from "@/components/ui/check-square";
import { PopoverMenu, PopoverMenuContent, PopoverMenuTrigger } from "@/components/ui/popover-menu";
import { usePermission } from "@/hooks/usePermission";
import { usePlacementSeeds } from "@/lib/api/hooks/app/placement/usePlacement";
import { cn } from "@/lib/utils";

const LAST_PICK_KEY = "warmbly.compose.seedPick";

function readLastPick(): string[] {
    try {
        const v = JSON.parse(localStorage.getItem(LAST_PICK_KEY) ?? "[]");
        return Array.isArray(v) ? v.filter((x): x is string => typeof x === "string") : [];
    } catch {
        return [];
    }
}

export default function SeedRecipientsButton({
    value,
    onChange,
}: {
    value: string[];
    onChange: (next: string[]) => void;
}) {
    const canView = usePermission("VIEW_CAMPAIGNS");
    const seedsQ = usePlacementSeeds(canView);
    const seeds = React.useMemo(
        () =>
            (seedsQ.data ?? [])
                .filter((s) => s.seed)
                .sort((a, b) => a.label.localeCompare(b.label) || a.email.localeCompare(b.email)),
        [seedsQ.data],
    );
    const [open, setOpen] = React.useState(false);
    const [picked, setPicked] = React.useState<string[]>([]);

    const added = React.useMemo(() => new Set(value.map((v) => v.trim().toLowerCase())), [value]);
    const addable = React.useMemo(() => seeds.filter((s) => !added.has(s.email.toLowerCase())), [seeds, added]);

    const families = React.useMemo(() => {
        const out: { family: string; label: string; emails: string[] }[] = [];
        for (const s of addable) {
            let f = out.find((x) => x.family === s.family);
            if (!f) {
                f = { family: s.family, label: s.label, emails: [] };
                out.push(f);
            }
            f.emails.push(s.email);
        }
        return out;
    }, [addable]);

    if (!canView || seeds.length === 0) return null;

    const openWith = (next: boolean) => {
        if (next) {
            const last = new Set(readLastPick());
            setPicked(addable.filter((s) => last.has(s.email_account_id)).map((s) => s.email));
        }
        setOpen(next);
    };
    const isPicked = (email: string) => picked.includes(email);
    const toggle = (email: string) =>
        setPicked((p) => (p.includes(email) ? p.filter((e) => e !== email) : [...p, email]));
    const toggleMany = (emails: string[]) => {
        const all = emails.every((e) => picked.includes(e));
        setPicked((p) => (all ? p.filter((e) => !emails.includes(e)) : [...p, ...emails.filter((e) => !p.includes(e))]));
    };
    const add = () => {
        onChange([...value, ...picked.filter((e) => !added.has(e.toLowerCase()))]);
        const ids = seeds.filter((s) => picked.includes(s.email)).map((s) => s.email_account_id);
        try {
            localStorage.setItem(LAST_PICK_KEY, JSON.stringify(ids));
        } catch {
            // Private mode or a full quota: the pick just is not remembered.
        }
        setOpen(false);
    };
    const onEscape = (e: React.KeyboardEvent) => {
        // Innermost layer only: the composer around it keeps its own Escape.
        if (e.key !== "Escape" || !open) return;
        e.preventDefault();
        e.stopPropagation();
        setOpen(false);
    };

    return (
        <PopoverMenu open={open} onOpenChange={openWith} align="end">
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    title="Add your seed inboxes"
                    onKeyDown={onEscape}
                    className="h-5 px-1 rounded text-[11px] text-slate-400 hover:text-slate-700 transition-colors"
                >
                    Seeds
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={300} className="w-[320px] py-0 overflow-hidden">
                <div onKeyDown={onEscape} className="flex flex-col max-h-[min(420px,70vh)]">
                    <div className="shrink-0 px-3 h-8 flex items-center gap-2 border-b border-slate-100">
                        <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Seed inboxes</span>
                        {addable.length > 1 && (
                            <button
                                type="button"
                                onClick={() => toggleMany(addable.map((s) => s.email))}
                                className="ml-auto h-5 px-1.5 rounded text-[11px] text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-colors"
                            >
                                {addable.every((s) => picked.includes(s.email)) ? "None" : "All"}
                            </button>
                        )}
                    </div>
                    {families.length > 1 && (
                        <div className="shrink-0 px-3 py-1.5 flex flex-wrap gap-1 border-b border-slate-100">
                            {families.map((f) => {
                                const all = f.emails.every((e) => picked.includes(e));
                                return (
                                    <button
                                        key={f.family}
                                        type="button"
                                        aria-pressed={all}
                                        onClick={() => toggleMany(f.emails)}
                                        className={cn(
                                            "h-5 px-1.5 rounded-md border text-[10.5px] font-medium inline-flex items-center gap-1 transition-colors",
                                            all
                                                ? "border-sky-200 bg-sky-50 text-sky-700"
                                                : "border-slate-200 bg-white text-slate-600 hover:border-slate-300",
                                        )}
                                    >
                                        {f.label}
                                        <span className="font-mono tabular-nums text-slate-400">{f.emails.length}</span>
                                    </button>
                                );
                            })}
                        </div>
                    )}
                    <div className="min-h-0 overflow-y-auto py-1">
                        {seeds.map((s) => {
                            const already = added.has(s.email.toLowerCase());
                            return (
                                <button
                                    key={s.email_account_id}
                                    type="button"
                                    disabled={already}
                                    onClick={() => toggle(s.email)}
                                    className={cn(
                                        "w-full h-7 px-3 flex items-center gap-2 text-left transition-colors",
                                        already ? "opacity-50" : "hover:bg-slate-50",
                                    )}
                                >
                                    <CheckSquare checked={already || isPicked(s.email)} />
                                    <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-slate-700">{s.email}</span>
                                    <span className="shrink-0 text-[10.5px] text-slate-400">{already ? "Added" : s.label}</span>
                                </button>
                            );
                        })}
                    </div>
                    <div className="shrink-0 px-3 py-2 border-t border-slate-100 flex items-center gap-2">
                        <p className="min-w-0 flex-1 text-[10.5px] leading-snug text-slate-400">
                            One message to every seed. For a copy each and a verdict per seed,{" "}
                            <Link to="/app/placement" onClick={() => setOpen(false)} className="text-sky-700 hover:underline">
                                run a placement test
                            </Link>
                            .
                        </p>
                        <button
                            type="button"
                            onClick={add}
                            disabled={picked.length === 0}
                            className="shrink-0 h-6 px-2.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[11.5px] font-medium transition-colors disabled:opacity-50"
                        >
                            Add{picked.length > 0 ? ` ${picked.length}` : ""}
                        </button>
                    </div>
                </div>
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
