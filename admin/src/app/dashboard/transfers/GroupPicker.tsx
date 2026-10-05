// Data group toggles for export and import. Ticking a group pulls in what it
// cannot travel without (as the server would); unticking one that others
// depend on explains itself instead of silently dropping them.

import { Checkbox } from "@/components/ui/checkbox";
import { StatusBadge } from "@/components/ui/kit";
import {
    dependentsOf,
    expandGroups,
    ORG_DATA_GROUP_CATALOG,
    type OrgDataGroup,
} from "@/lib/api/client/admin/transfers";
import { cn } from "@/lib/utils";

export function GroupPicker({
    selected,
    onChange,
    /** Restrict to these keys (an archive's contents on import). */
    available,
    disabled,
}: {
    selected: Set<OrgDataGroup>;
    onChange: (next: Set<OrgDataGroup>) => void;
    available?: OrgDataGroup[];
    disabled?: boolean;
}) {
    const groups = available
        ? ORG_DATA_GROUP_CATALOG.filter((g) => available.includes(g.key))
        : ORG_DATA_GROUP_CATALOG;

    function toggle(key: OrgDataGroup) {
        const next = new Set(selected);
        if (next.has(key)) {
            next.delete(key);
            for (const d of dependentsOf(key, next)) next.delete(d.key);
        } else {
            next.add(key);
        }
        onChange(expandGroups(next));
    }

    return (
        <div className="grid gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-2">
            {groups.map((g) => {
                const on = g.required || selected.has(g.key);
                const deps = dependentsOf(g.key, selected);
                return (
                    <label
                        key={g.key}
                        className={cn(
                            "flex items-start gap-2.5 px-3 py-2.5 text-[12.5px] transition-colors",
                            on ? "bg-card" : "bg-[color-mix(in_oklab,var(--card)_94%,var(--muted-foreground))]",
                            g.required || disabled ? "cursor-default" : "cursor-pointer hover:bg-accent",
                        )}
                    >
                        <Checkbox
                            checked={on}
                            disabled={g.required || disabled}
                            onCheckedChange={() => toggle(g.key)}
                            className="mt-0.5"
                        />
                        <span className="min-w-0 flex-1">
                            <span className="flex flex-wrap items-center gap-1.5">
                                <span
                                    className={cn(
                                        "text-[13px] font-medium leading-tight",
                                        on ? "text-foreground" : "text-muted-foreground",
                                    )}
                                >
                                    {g.label}
                                </span>
                                {g.required && <StatusBadge className="h-4 px-1.5 text-[10.5px]">required</StatusBadge>}
                                {g.heavy && (
                                    <StatusBadge tone="warning" className="h-4 px-1.5 text-[10.5px]">
                                        heavy
                                    </StatusBadge>
                                )}
                            </span>
                            <span className="mt-0.5 block text-xs leading-snug text-muted-foreground">{g.description}</span>
                            {on && deps.length > 0 && !g.required && (
                                <span className="mt-0.5 block text-[11px] leading-snug text-subtle-foreground">
                                    Needed by {deps.map((d) => d.label).join(", ")}.
                                </span>
                            )}
                        </span>
                    </label>
                );
            })}
            {groups.length % 2 === 1 && <div aria-hidden className="hidden bg-card sm:block" />}
        </div>
    );
}
