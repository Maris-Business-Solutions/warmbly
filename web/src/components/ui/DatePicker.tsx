// DatePicker — the house date field: a themed trigger + the portaled Calendar
// popover, replacing the native <input type="date"> (and its browser-chrome
// calendar) everywhere in the dashboard. Value is a "yyyy-MM-dd" string (the
// same shape the native input used) so it's a drop-in for existing filters.

import React from "react";
import { format, parse, isValid } from "date-fns";
import { CalendarIcon, XIcon } from "lucide-react";

import { cn } from "@/lib/utils";
import Calendar from "@/components/app/Calendar";

// Parse "yyyy-MM-dd" as a LOCAL date (avoids the UTC shift new Date("yyyy-MM-dd")
// introduces, which can land the picker a day off in negative-offset timezones).
function parseISODate(v: string): Date | null {
    if (!v) return null;
    const d = parse(v, "yyyy-MM-dd", new Date());
    return isValid(d) ? d : null;
}

export function DatePicker({
    value,
    onChange,
    placeholder = "Any date",
    className,
    clearable = true,
    disabled = false,
    display = "MMM d, yyyy",
}: {
    /** "yyyy-MM-dd", or "" when unset. */
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    className?: string;
    clearable?: boolean;
    disabled?: boolean;
    /** date-fns format for the trigger label. */
    display?: string;
}) {
    const [open, setOpen] = React.useState(false);
    const wrapRef = React.useRef<HTMLDivElement>(null);
    const selected = parseISODate(value);

    // Close on outside click. BUBBLE phase (not capture): the Calendar panel is
    // portaled to <body> and stops mousedown bubbling, so its own clicks (month
    // nav, day select) never reach here and don't dismiss it mid-interaction.
    React.useEffect(() => {
        if (!open) return;
        const onDown = (e: MouseEvent | TouchEvent) => {
            if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
        };
        document.addEventListener("mousedown", onDown);
        document.addEventListener("touchstart", onDown);
        return () => {
            document.removeEventListener("mousedown", onDown);
            document.removeEventListener("touchstart", onDown);
        };
    }, [open]);

    // Escape closes the calendar only: stopped in the window capture phase so
    // the popover or dialog holding this field stays open.
    React.useEffect(() => {
        if (!open) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "Escape") return;
            e.stopPropagation();
            setOpen(false);
        };
        window.addEventListener("keydown", onKey, true);
        return () => window.removeEventListener("keydown", onKey, true);
    }, [open]);

    return (
        <div ref={wrapRef} className={cn("relative inline-flex", className)}>
            <button
                type="button"
                disabled={disabled}
                onClick={() => setOpen((o) => !o)}
                className={cn(
                    "h-7 w-full px-2 rounded-md border bg-white inline-flex items-center gap-1.5 text-[12px] transition-colors disabled:opacity-60 disabled:cursor-not-allowed",
                    open ? "border-sky-400 ring-2 ring-sky-100" : "border-slate-200 hover:border-slate-300",
                )}
            >
                <CalendarIcon className="w-3 h-3 text-slate-400 shrink-0" />
                <span className={cn("truncate flex-1 text-left", selected ? "text-slate-900" : "text-slate-400")}>
                    {selected ? format(selected, display) : placeholder}
                </span>
                {clearable && value && (
                    <span
                        role="button"
                        tabIndex={-1}
                        aria-label="Clear date"
                        onClick={(e) => {
                            e.stopPropagation();
                            onChange("");
                        }}
                        className="text-slate-400 hover:text-slate-700 shrink-0"
                    >
                        <XIcon className="w-3 h-3" />
                    </span>
                )}
            </button>
            <Calendar
                date={selected}
                active={open}
                close={() => setOpen(false)}
                onSubmit={(d) => onChange(d ? format(d, "yyyy-MM-dd") : "")}
            />
        </div>
    );
}

export default DatePicker;
