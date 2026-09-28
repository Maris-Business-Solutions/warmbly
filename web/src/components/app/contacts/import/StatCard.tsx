import type { LucideIcon } from "lucide-react";
import AnimatedNumber from "@/components/ui/AnimatedNumber";

export type StatAccent = "emerald" | "sky" | "slate" | "red" | "amber";

const ACCENTS: Record<StatAccent, string> = {
    emerald: "ring-emerald-200 bg-emerald-50 text-emerald-700",
    sky: "ring-sky-200 bg-sky-50 text-sky-700",
    slate: "ring-slate-200 bg-slate-50 text-slate-700",
    red: "ring-red-200 bg-red-50 text-red-700",
    amber: "ring-amber-200 bg-amber-50 text-amber-800",
};

// StatCard is one count of an import, ticking up as a running import settles.
export default function StatCard({
    label,
    value,
    accent,
    icon: Icon,
    hint,
}: {
    label: string;
    value: number;
    accent: StatAccent;
    icon: LucideIcon;
    hint?: string;
}) {
    return (
        <div className={`rounded-md ring-1 p-2.5 ${ACCENTS[accent]}`} title={hint}>
            <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-[0.14em] font-medium opacity-80">
                <Icon className="w-3 h-3 shrink-0" />
                <span className="truncate">{label}</span>
            </div>
            <AnimatedNumber value={value} className="block text-[18px] font-semibold tabular-nums mt-0.5" />
            {hint && <p className="text-[10.5px] leading-snug opacity-75 mt-0.5 line-clamp-2">{hint}</p>}
        </div>
    );
}
