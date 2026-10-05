// Segmented status filter with a count per option: the kit Segmented look,
// plus a quiet count after each label.

import { cn } from "@/lib/utils";

export function StatusSegments<T extends string>({
    value,
    onChange,
    options,
}: {
    value: T;
    onChange: (v: T) => void;
    options: { value: T; label: string; count?: number }[];
}) {
    return (
        <div className="inline-flex h-7 max-w-full items-center overflow-x-auto rounded-md border border-border bg-muted/50 p-0.5 no-scrollbar">
            {options.map((o) => {
                const active = value === o.value;
                return (
                    <button
                        key={o.value}
                        type="button"
                        aria-pressed={active}
                        onClick={() => onChange(o.value)}
                        className={cn(
                            "inline-flex h-full shrink-0 items-center gap-1.5 rounded-[5px] px-2 text-[12px] font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                            active
                                ? "bg-card text-foreground shadow-[0_1px_2px_rgb(0_0_0/0.08)] dark:bg-accent"
                                : "text-muted-foreground hover:text-foreground",
                        )}
                    >
                        {o.label}
                        {o.count !== undefined && (
                            <span
                                className={cn(
                                    "text-[11px] tabular-nums",
                                    active ? "text-muted-foreground" : "text-subtle-foreground",
                                )}
                            >
                                {o.count.toLocaleString()}
                            </span>
                        )}
                    </button>
                );
            })}
        </div>
    );
}
