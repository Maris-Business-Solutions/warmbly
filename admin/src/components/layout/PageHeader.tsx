// The page's header bar: a 48px strip stuck to the top of the panel holding
// the section icon, breadcrumbs, the title and the page's actions, with the
// optional description as a quiet line underneath. A page in a section with
// several pages shows the section's pages as tabs here instead of a title.
//
// It must be the FIRST element a page renders: it bleeds to the panel edges
// with the shell's --page-x / --page-y, which only lines up from the top.

import { Fragment, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { motion } from "motion/react";
import { ChevronRight, type LucideIcon } from "lucide-react";
import { useMe } from "@/hooks/useMe";
import { hasAdminPerm } from "@/lib/auth/permissions";
import { cn } from "@/lib/utils";
import { findSection, isSectionPage, pageMatches, type NavIcon } from "./nav";

export interface Crumb {
    label: string;
    to: string;
}

interface PageHeaderProps {
    title: ReactNode;
    description?: ReactNode;
    // Ancestors before the title, e.g. [{ label: "Users", to: "/users" }].
    breadcrumbs?: Crumb[];
    // Overrides the section icon looked up from the nav model.
    icon?: LucideIcon;
    // Inline after the title: a count, a status badge.
    meta?: ReactNode;
    // Right-aligned actions. Use size="sm" buttons so they fit the bar.
    children?: ReactNode;
    className?: string;
}

export function PageHeader({ title, description, breadcrumbs, icon, meta, children, className }: PageHeaderProps) {
    const { pathname } = useLocation();
    const { data: me } = useMe();
    const section = findSection(pathname);
    const Icon: NavIcon | undefined = icon ?? section?.icon;
    const tabs = (section?.pages ?? []).filter(
        (p) => p.perm === undefined || hasAdminPerm(me?.admin_permissions, p.perm),
    );
    const showTabs = !breadcrumbs?.length && tabs.length > 1 && tabs.some((t) => isSectionPage(t, pathname));

    return (
        <>
            <header
                className={cn(
                    "sticky top-0 z-20 -mx-[var(--page-x)] -mt-[var(--page-y)] flex min-h-12 flex-wrap items-center gap-x-3 gap-y-2 border-b border-border bg-background/85 px-[var(--page-x)] py-2 backdrop-blur-md supports-[backdrop-filter]:bg-background/75",
                    description ? "mb-0" : "mb-[var(--page-y)]",
                    className,
                )}
            >
                <div className="flex min-w-0 flex-1 items-center gap-1.5 text-[13px]">
                    {Icon && <Icon className="size-4 shrink-0 text-subtle-foreground" />}
                    {breadcrumbs?.map((c) => (
                        <Fragment key={c.to}>
                            <Link
                                to={c.to}
                                className="shrink-0 rounded px-1 py-0.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                            >
                                {c.label}
                            </Link>
                            <ChevronRight className="size-3.5 shrink-0 text-subtle-foreground" />
                        </Fragment>
                    ))}
                    {showTabs && section ? (
                        <>
                            <span className="shrink-0 px-0.5 font-medium text-foreground">{section.label}</span>
                            <span className="mx-1 h-4 w-px shrink-0 bg-border-strong" />
                            {/* The page title is the active tab; keep it for screen readers. */}
                            <h1 className="sr-only">{title}</h1>
                            <nav className="no-scrollbar -my-1 flex min-w-0 items-center gap-0.5 overflow-x-auto py-1">
                                {tabs.map((t) => {
                                    const active = pageMatches(t, pathname);
                                    return (
                                        <Link
                                            key={t.to}
                                            to={t.to}
                                            aria-current={active ? "page" : undefined}
                                            className={cn(
                                                "relative h-7 shrink-0 rounded-md px-2.5 leading-7 whitespace-nowrap transition-colors duration-150 outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                                                active ? "text-foreground" : "text-muted-foreground hover:text-foreground",
                                            )}
                                        >
                                            {active && (
                                                <motion.span
                                                    layoutId="header-tab"
                                                    transition={{ type: "spring", stiffness: 520, damping: 42, mass: 0.7 }}
                                                    className="absolute inset-0 rounded-md bg-accent shadow-[inset_0_0_0_1px_var(--border)]"
                                                />
                                            )}
                                            <span className="relative">{t.label}</span>
                                        </Link>
                                    );
                                })}
                            </nav>
                        </>
                    ) : (
                        <h1 className={cn("truncate font-medium text-foreground", breadcrumbs?.length ? "px-1" : "px-0.5")}>
                            {title}
                        </h1>
                    )}
                    {meta && <div className="flex shrink-0 items-center gap-1.5">{meta}</div>}
                </div>
                {children && <div className="flex flex-wrap items-center gap-1.5">{children}</div>}
            </header>
            {description && (
                <p className="mt-3 mb-[var(--page-y)] max-w-3xl text-[13px] leading-relaxed text-muted-foreground">
                    {description}
                </p>
            )}
        </>
    );
}
