// The one status palette. Pills, legends and inline state text read from
// here so every state looks the same in light and dark. Never hand-write
// `bg-emerald-50 text-emerald-700` in a page.

export type Tone = "neutral" | "accent" | "success" | "warning" | "orange" | "danger" | "info" | "strong";

// Tinted pill: border + wash + text. Pair with <Badge variant="outline">.
export const TONE: Record<Tone, string> = {
    neutral: "border-border-strong bg-muted/60 text-muted-foreground",
    accent: "border-border-strong bg-[var(--admin-accent-weak)] text-foreground",
    success: "border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
    warning: "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400",
    orange: "border-orange-500/30 bg-orange-500/10 text-orange-700 dark:text-orange-400",
    danger: "border-red-500/25 bg-red-500/10 text-red-700 dark:text-red-400",
    info: "border-sky-500/25 bg-sky-500/10 text-sky-700 dark:text-sky-400",
    strong: "border-border-strong bg-[var(--admin-accent-weak)] text-foreground",
};

// Foreground only, for a number or a word that carries state.
export const TONE_TEXT: Record<Tone, string> = {
    neutral: "text-muted-foreground",
    accent: "text-[var(--admin-accent-strong)]",
    success: "text-emerald-600 dark:text-emerald-400",
    warning: "text-amber-600 dark:text-amber-400",
    orange: "text-orange-600 dark:text-orange-400",
    danger: "text-red-600 dark:text-red-400",
    info: "text-sky-600 dark:text-sky-400",
    strong: "text-foreground",
};

// Solid fill, for status dots and bar segments.
export const TONE_DOT: Record<Tone, string> = {
    neutral: "bg-subtle-foreground",
    accent: "bg-[var(--admin-accent)]",
    success: "bg-emerald-500",
    warning: "bg-amber-500",
    orange: "bg-orange-500",
    danger: "bg-red-500",
    info: "bg-sky-500",
    strong: "bg-foreground",
};

// Callout panel: a bordered wash for banners and inline notices.
export const TONE_PANEL: Record<Tone, string> = {
    neutral: "border-border bg-muted/40 text-foreground",
    accent: "border-border-strong bg-[var(--admin-accent-weak)] text-foreground",
    success: "border-emerald-500/20 bg-emerald-500/[0.06] text-foreground",
    warning: "border-amber-500/25 bg-amber-500/[0.07] text-foreground",
    orange: "border-orange-500/25 bg-orange-500/[0.07] text-foreground",
    danger: "border-red-500/20 bg-red-500/[0.06] text-foreground",
    info: "border-sky-500/20 bg-sky-500/[0.06] text-foreground",
    strong: "border-border-strong bg-muted/40 text-foreground",
};
