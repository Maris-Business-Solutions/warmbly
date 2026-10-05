// Long free text in a table cell (an error message, a failure reason):
// truncated with the full text as a tooltip, and a toggle to show it all.

import { useState } from "react";
import { cn } from "@/lib/utils";

export function ExpandableText({
    text,
    max = 90,
    className,
    mono,
}: {
    text: string;
    max?: number;
    className?: string;
    mono?: boolean;
}) {
    const [open, setOpen] = useState(false);
    if (!text) return <span className="text-xs text-subtle-foreground">—</span>;
    const long = text.length > max;
    const shown = open || !long ? text : `${text.slice(0, max).trimEnd()}…`;
    return (
        <span className={cn("text-xs leading-relaxed", mono && "font-mono text-[11.5px]", className)}>
            <span className={open ? "whitespace-pre-wrap break-words" : "break-words"} title={long && !open ? text : undefined}>
                {shown}
            </span>
            {long && (
                <button
                    type="button"
                    onClick={(e) => {
                        e.stopPropagation();
                        setOpen((v) => !v);
                    }}
                    className="ml-1.5 rounded-[4px] px-0.5 font-sans text-[11.5px] font-medium text-[var(--admin-accent-strong)] outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring/40"
                >
                    {open ? "less" : "more"}
                </button>
            )}
        </span>
    );
}
