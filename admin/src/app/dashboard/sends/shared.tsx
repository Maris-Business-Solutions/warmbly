// Components the Sends tabs share: the workspace link cell and the intro row
// above each tab.

import type { ReactNode } from "react";
import { Link } from "react-router-dom";

export function WorkspaceLink({ id, name }: { id?: string | null; name?: string | null }) {
    if (!id) return <span className="text-xs text-subtle-foreground">—</span>;
    return (
        <Link
            to={`/organizations/${id}`}
            onClick={(e) => e.stopPropagation()}
            className="text-[13px] text-foreground decoration-border-strong underline-offset-2 hover:underline"
        >
            {name || id}
        </Link>
    );
}

export function TabIntro({ children, actions }: { children: ReactNode; actions?: ReactNode }) {
    return (
        <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <p className="max-w-2xl text-[13px] leading-relaxed text-muted-foreground">{children}</p>
            {actions && <div className="flex shrink-0 items-center gap-1.5">{actions}</div>}
        </div>
    );
}
