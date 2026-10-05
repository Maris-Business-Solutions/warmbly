import { cn } from "@/lib/utils";
import { ENV_LABEL, type EnvLabel } from "@/lib/env";

// Environment indicator at the foot of the sidebar, so a production admin
// window is told apart from a staging one at a glance. Production is the
// only one drawn in a warning colour.

const DOT: Record<EnvLabel, string> = {
    production: "bg-red-500 shadow-[0_0_0_3px_rgb(239_68_68/0.18)]",
    staging: "bg-amber-500",
    development: "bg-emerald-500",
};

const LABELS: Record<EnvLabel, string> = {
    production: "Production",
    staging: "Staging",
    development: "Development",
};

export function EnvPill({ className }: { className?: string }) {
    return (
        <span
            className={cn(
                "inline-flex h-6 items-center gap-1.5 rounded-md px-1.5 text-[12px] font-medium text-sidebar-foreground",
                className,
            )}
            title={`Connected to ${LABELS[ENV_LABEL]} environment`}
        >
            <span className={cn("size-1.5 rounded-full", DOT[ENV_LABEL])} />
            {LABELS[ENV_LABEL]}
        </span>
    );
}
