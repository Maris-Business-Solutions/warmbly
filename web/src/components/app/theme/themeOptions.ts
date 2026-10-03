import type React from "react";
import { MonitorIcon, MoonIcon, SunIcon } from "lucide-react";
import type { Theme, ThemeOrigin } from "@/lib/theme";

export const THEME_OPTIONS: { value: Theme; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
    { value: "light", label: "Light", icon: SunIcon },
    { value: "dark", label: "Dark", icon: MoonIcon },
    { value: "system", label: "System", icon: MonitorIcon },
];

// A keyboard activation has no pointer position, so the reveal starts at the control.
export function originOf(e: React.MouseEvent | React.KeyboardEvent): ThemeOrigin {
    if ("clientX" in e && (e.clientX || e.clientY)) return { x: e.clientX, y: e.clientY };
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
}
