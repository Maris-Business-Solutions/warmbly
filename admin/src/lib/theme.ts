// Light / dark / system theme, dark by default. index.html applies the stored
// choice before first paint; this module keeps <html class="dark"> in step.

import { useSyncExternalStore } from "react";

export type ThemePref = "system" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

const KEY = "warmbly-admin:theme";
const media = typeof window !== "undefined" ? window.matchMedia("(prefers-color-scheme: dark)") : null;
const listeners = new Set<() => void>();

function readPref(): ThemePref {
    try {
        const v = localStorage.getItem(KEY);
        return v === "light" || v === "system" ? v : "dark";
    } catch {
        return "dark";
    }
}

let pref: ThemePref = readPref();

function resolve(p: ThemePref): ResolvedTheme {
    if (p === "system") return media?.matches ? "dark" : "light";
    return p;
}

function apply() {
    const dark = resolve(pref) === "dark";
    document.documentElement.classList.toggle("dark", dark);
    listeners.forEach((l) => l());
}

media?.addEventListener("change", () => {
    if (pref === "system") apply();
});

export function setThemePref(next: ThemePref) {
    pref = next;
    try {
        localStorage.setItem(KEY, next);
    } catch {
        /* private mode: the choice lasts for the tab */
    }
    apply();
}

function subscribe(l: () => void) {
    listeners.add(l);
    return () => {
        listeners.delete(l);
    };
}

export function useTheme(): { pref: ThemePref; resolved: ResolvedTheme; setPref: (p: ThemePref) => void } {
    const snapshot = useSyncExternalStore(
        subscribe,
        () => `${pref}:${resolve(pref)}`,
        () => "dark:dark",
    );
    const [p, r] = snapshot.split(":") as [ThemePref, ResolvedTheme];
    return { pref: p, resolved: r, setPref: setThemePref };
}
