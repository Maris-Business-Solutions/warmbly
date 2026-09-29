// The checkbox square of the app's pickers (category, tag and column
// choosers): a filled square with a white check when on, a bordered white
// square when off. Purely visual; the row it sits in is the control.
// Tones match Checkbox: slate for option toggles, sky for choosing what shows.

import { CheckIcon } from "lucide-react";
import { cn } from "@/lib/utils";

const TONE = {
    slate: "border-slate-900 bg-slate-900",
    sky: "border-sky-600 bg-sky-600",
} as const;

export function CheckSquare({ checked, tone = "slate" }: { checked: boolean; tone?: keyof typeof TONE }) {
    return (
        <span
            className={cn(
                "size-3.5 rounded border flex items-center justify-center transition-colors shrink-0",
                checked ? TONE[tone] : "border-slate-300 bg-white",
            )}
        >
            {checked && <CheckIcon className="w-2 h-2 text-white" strokeWidth={3.5} />}
        </span>
    );
}
