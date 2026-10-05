// The workspace mark at the top of the sidebar and the mobile drawer.

import { Logo } from "@/components/Logo";

export function SidebarMark() {
    return (
        <span className="grid size-[22px] shrink-0 place-items-center rounded-[6px] bg-foreground text-background">
            <Logo className="size-3.5" />
        </span>
    );
}

export function SidebarBrand() {
    return (
        <div className="flex min-w-0 items-center gap-2">
            <SidebarMark />
            <span className="truncate text-[13px] font-semibold text-sidebar-accent-foreground">Warmbly</span>
            <span className="rounded-[4px] bg-sidebar-accent px-1 py-px text-[10.5px] font-medium text-sidebar-foreground">
                Admin
            </span>
        </div>
    );
}
