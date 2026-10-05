import { ShieldAlert } from "lucide-react";
import { cn } from "@/lib/utils";

// The "Admin" marker on the sign-in and gate screens, so an admin who
// switches tabs from the dashboard sees an elevated-privilege surface.

interface AdminBadgeProps {
    className?: string;
    compact?: boolean;
}

export function AdminBadge({ className, compact = false }: AdminBadgeProps) {
    return (
        <span
            className={cn(
                "inline-flex items-center gap-1 rounded-md border font-medium",
                "border-[color-mix(in_oklab,var(--admin-accent)_30%,transparent)] bg-[var(--admin-accent-weak)] text-[var(--admin-accent-strong)]",
                compact ? "px-1.5 py-px text-[11px]" : "px-2 py-0.5 text-xs",
                className,
            )}
        >
            <ShieldAlert className={compact ? "size-3" : "size-3.5"} />
            Admin
        </span>
    );
}
