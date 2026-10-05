// An admin's bits as chips grouped by catalog category. Bits the catalog does
// not know (retired ones still stored on old super admins) are not shown.

import { StatusBadge } from "@/components/ui/kit";
import type { PermissionInfo } from "@/lib/api/client/admin/admins";
import { groupByCategory, hasBit, humanize } from "./permissions";

export function PermissionChips({ mask, catalog }: { mask: number; catalog: PermissionInfo[] }) {
    const groups = groupByCategory(catalog)
        .map((g) => ({ ...g, items: g.items.filter((p) => hasBit(mask, p.permission)) }))
        .filter((g) => g.items.length > 0);
    if (groups.length === 0) {
        return <span className="text-xs text-muted-foreground">No live permissions</span>;
    }
    return (
        <div className="flex flex-col gap-1 py-1.5">
            {groups.map((g) => (
                <div key={g.category} className="flex flex-wrap items-center gap-1">
                    <span className="mr-1 min-w-16 text-xs font-medium text-muted-foreground">{g.category}</span>
                    {g.items.map((p) => (
                        <StatusBadge
                            key={p.name}
                            tone={p.name === "grant_admin_access" ? "accent" : "neutral"}
                            title={p.description}
                            className="h-[18px] px-1.5 text-[11px]"
                        >
                            {humanize(p.name)}
                        </StatusBadge>
                    ))}
                </div>
            ))}
        </div>
    );
}
