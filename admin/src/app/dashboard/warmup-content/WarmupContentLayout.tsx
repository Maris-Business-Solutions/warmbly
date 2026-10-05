// Warmup content section shell: the one page header, route-linked view pills
// (styled like PageTabs) and an <Outlet/> for the active view. Child pages
// render no header of their own.
//
//   /warmup-content/overview   headline counts, AI/schedule status, A/B
//   /warmup-content/library    generated-thread library + actions
//   /warmup-content/jobs       generation jobs, polled live

import { NavLink, Outlet } from "react-router-dom";
import { Inbox, Layers, Play, type LucideIcon } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { cn } from "@/lib/utils";

interface SubTab {
    to: string;
    label: string;
    icon: LucideIcon;
}

const TABS: SubTab[] = [
    { to: "/warmup-content/overview", label: "Overview", icon: Layers },
    { to: "/warmup-content/library", label: "Library", icon: Inbox },
    { to: "/warmup-content/jobs", label: "Jobs", icon: Play },
];

export default function WarmupContentLayout() {
    return (
        <div className="w-full">
            <PageHeader
                title="Warmup content"
                description="Observe the autonomous warmup-content controller, review its generated library, and inspect generation history. Content volume and refresh are managed from live warmup demand."
            />

            <nav aria-label="Warmup content views" className="no-scrollbar mb-5 flex shrink-0 items-center gap-1 overflow-x-auto">
                {TABS.map(({ to, label, icon: Icon }) => (
                    <NavLink
                        key={to}
                        to={to}
                        className={({ isActive }) =>
                            cn(
                                "inline-flex h-7 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-md border px-2.5 text-[12.5px] font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                                isActive
                                    ? "border-border-strong bg-accent text-foreground"
                                    : "border-transparent text-muted-foreground hover:bg-accent/60 hover:text-foreground",
                            )
                        }
                    >
                        {({ isActive }) => (
                            <>
                                <Icon className={cn("size-3.5 shrink-0", !isActive && "text-subtle-foreground")} />
                                {label}
                            </>
                        )}
                    </NavLink>
                ))}
            </nav>

            <Outlet />
        </div>
    );
}
