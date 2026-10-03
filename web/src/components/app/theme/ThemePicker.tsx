// The appearance choice in two sizes: preview cards for Settings and a
// three-way switch for the user menu. Both are radio groups, and both start the
// new theme from wherever it was picked.

import React from "react";
import { CheckIcon } from "lucide-react";
import { useAppStore } from "@/stores";
import type { Theme, ThemeOrigin } from "@/lib/theme";
import { cn } from "@/lib/utils";
import { THEME_OPTIONS, originOf } from "./themeOptions";

// Arrow keys move the choice, as in any radio group.
function useRadioKeys(value: Theme, choose: (t: Theme, origin: ThemeOrigin) => void) {
    return (e: React.KeyboardEvent<HTMLElement>) => {
        const step = e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : e.key === "ArrowLeft" || e.key === "ArrowUp" ? -1 : 0;
        if (!step) return;
        e.preventDefault();
        // Inside a menu, the arrows belong to this group rather than the menu's item focus.
        e.stopPropagation();
        const i = THEME_OPTIONS.findIndex((o) => o.value === value);
        const next = THEME_OPTIONS[(i + step + THEME_OPTIONS.length) % THEME_OPTIONS.length];
        const group = e.currentTarget;
        const button = group.querySelector<HTMLElement>(`[data-theme-option="${next.value}"]`);
        button?.focus();
        const r = button?.getBoundingClientRect();
        choose(next.value, r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : { x: 0, y: 0 });
    };
}

/** Compact segmented switch, sized for a menu row. */
export function ThemeSwitch({ className }: { className?: string }) {
    const theme = useAppStore((s) => s.theme);
    const setTheme = useAppStore((s) => s.setTheme);
    const onKeyDown = useRadioKeys(theme, setTheme);

    return (
        <div
            role="radiogroup"
            aria-label="Theme"
            onKeyDown={onKeyDown}
            className={cn("inline-flex items-center gap-0.5 rounded-md bg-slate-100 p-0.5", className)}
        >
            {THEME_OPTIONS.map(({ value, label, icon: Icon }) => {
                const active = theme === value;
                return (
                    <button
                        key={value}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        aria-label={label}
                        title={label}
                        tabIndex={active ? 0 : -1}
                        data-theme-option={value}
                        onClick={(e) => {
                            e.stopPropagation();
                            setTheme(value, originOf(e));
                        }}
                        className={cn(
                            "inline-flex h-6 w-7 items-center justify-center rounded-[5px] outline-none transition-colors focus-visible:ring-2 focus-visible:ring-sky-200",
                            active
                                ? "bg-white text-slate-900 shadow-[0_1px_2px_rgba(15,23,42,0.08)] ring-1 ring-slate-200/70"
                                : "text-slate-400 hover:text-slate-700",
                        )}
                    >
                        <Icon className="size-3.5" />
                    </button>
                );
            })}
        </div>
    );
}

/** Settings cards, each a miniature of the dashboard in that theme. */
export function ThemeCards() {
    const theme = useAppStore((s) => s.theme);
    const resolved = useAppStore((s) => s.resolvedTheme);
    const setTheme = useAppStore((s) => s.setTheme);
    const onKeyDown = useRadioKeys(theme, setTheme);

    return (
        <div role="radiogroup" aria-label="Theme" onKeyDown={onKeyDown} className="grid grid-cols-1 gap-3 min-[460px]:grid-cols-3 max-w-[600px]">
            {THEME_OPTIONS.map(({ value, label, icon: Icon }) => {
                const active = theme === value;
                return (
                    <button
                        key={value}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        tabIndex={active ? 0 : -1}
                        data-theme-option={value}
                        onClick={(e) => setTheme(value, originOf(e))}
                        className={cn(
                            "group rounded-lg border p-1.5 text-left outline-none transition-[border-color,box-shadow] focus-visible:ring-2 focus-visible:ring-sky-200",
                            active
                                ? "border-sky-500 ring-2 ring-sky-100"
                                : "border-slate-200 hover:border-slate-300",
                        )}
                    >
                        <ThemeThumb variant={value} />
                        <span className="flex items-center gap-1.5 px-1 pb-0.5 pt-2 text-[12.5px]">
                            <Icon className={cn("size-3.5", active ? "text-sky-600" : "text-slate-400")} />
                            <span className={active ? "font-medium text-slate-900" : "text-slate-700"}>{label}</span>
                            {value === "system" && (
                                <span className="text-[11px] text-slate-400">({resolved === "dark" ? "dark" : "light"} now)</span>
                            )}
                            <span
                                aria-hidden
                                className={cn(
                                    "ml-auto inline-flex size-4 items-center justify-center rounded-full transition-[opacity,transform] duration-200",
                                    active ? "scale-100 bg-sky-600 opacity-100" : "scale-75 opacity-0",
                                )}
                            >
                                <CheckIcon className="size-2.5 text-white" strokeWidth={3.5} />
                            </span>
                        </span>
                    </button>
                );
            })}
        </div>
    );
}

// Fixed colours, not palette classes: a preview shows its own theme whichever one is on.
const THUMB = {
    light: { chrome: "#f5f6f8", panel: "#ffffff", line: "#e2e8f0", strong: "#cbd5e1", text: "#94a3b8", head: "#334155", accent: "#0284c7" },
    dark: { chrome: "#070708", panel: "#111214", line: "#26272b", strong: "#35373c", text: "#62666d", head: "#d0d6e0", accent: "#0284c7" },
} as const;

function Miniature({ c }: { c: (typeof THUMB)[keyof typeof THUMB] }) {
    return (
        <div className="absolute inset-0 flex" style={{ background: c.chrome }}>
            <div className="flex w-[26%] flex-col gap-1.5 px-2 pt-3">
                <span className="mb-1 h-1.5 w-3/4 rounded-full" style={{ background: c.strong }} />
                <span className="h-1 w-full rounded-full" style={{ background: c.line }} />
                <span className="h-1 w-4/5 rounded-full" style={{ background: c.line }} />
                <span className="h-1 w-full rounded-full" style={{ background: c.line }} />
            </div>
            <div
                className="mt-2 flex flex-1 flex-col gap-1.5 rounded-tl-md border-l border-t px-2.5 pt-2.5"
                style={{ background: c.panel, borderColor: c.line }}
            >
                <div className="flex items-center justify-between">
                    <span className="h-1.5 w-1/3 rounded-full" style={{ background: c.head }} />
                    <span className="h-2.5 w-6 rounded-[3px]" style={{ background: c.accent }} />
                </div>
                {[0.9, 0.7, 0.8].map((w, i) => (
                    <div key={i} className="flex items-center gap-1.5 border-t pt-1.5" style={{ borderColor: c.line }}>
                        <span className="size-1.5 shrink-0 rounded-full" style={{ background: i === 1 ? "#10b981" : c.strong }} />
                        <span className="h-1 rounded-full" style={{ width: `${w * 70}%`, background: c.text }} />
                    </div>
                ))}
            </div>
        </div>
    );
}

function ThemeThumb({ variant }: { variant: Theme }) {
    return (
        <span aria-hidden className="relative block h-[84px] overflow-hidden rounded-md ring-1 ring-black/5">
            <Miniature c={variant === "dark" ? THUMB.dark : THUMB.light} />
            {variant === "system" && (
                <span className="absolute inset-0" style={{ clipPath: "polygon(58% 0, 100% 0, 100% 100%, 42% 100%)" }}>
                    <Miniature c={THUMB.dark} />
                </span>
            )}
        </span>
    );
}
