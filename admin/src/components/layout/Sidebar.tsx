// Left rail navigation: the brand and search on top, collapsible sections of
// dense icon rows, then the instance status and the signed-in account. The rail
// itself folds down to icons (button or the `[` key) and remembers it.
// The nav model lives in nav.ts.

import { useEffect, useId, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { SidebarBrand, SidebarMark } from "./Brand";
import { AnimatePresence, LayoutGroup, motion } from "motion/react";
import { ChevronRight, PanelLeft, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useMe } from "@/hooks/useMe";
import { AdminPerm, hasAdminPerm } from "@/lib/auth/permissions";
import { sectionActive, visibleNavGroups, type NavItem } from "./nav";
import { findingCount, useInstanceHealth, worstSeverity } from "@/hooks/useInstanceHealth";
import type { CheckSeverity } from "@/lib/api/client/admin/instance";
import { IS_MAC, openCommandPalette } from "./CommandPalette";
import { EnvPill } from "./EnvPill";
import { UpdatePill } from "./UpdatePill";
import { UserMenu } from "./UserMenu";

const RAIL_KEY = "warmbly-admin:sidebar";
const SPRING = { type: "spring", stiffness: 520, damping: 42, mass: 0.7 } as const;
const EASE = [0.22, 1, 0.36, 1] as const;

function readRail(): boolean {
    try {
        return localStorage.getItem(RAIL_KEY) === "rail";
    } catch {
        return false;
    }
}

function isTyping(el: EventTarget | null): boolean {
    if (!(el instanceof HTMLElement)) return false;
    return el.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(el.tagName);
}

export function Sidebar() {
    const [rail, setRail] = useState(readRail);

    function toggleRail() {
        setRail((v) => {
            try {
                localStorage.setItem(RAIL_KEY, v ? "expanded" : "rail");
            } catch {
                /* ignore */
            }
            return !v;
        });
    }

    useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if (e.key !== "[" || e.metaKey || e.ctrlKey || e.altKey || isTyping(e.target)) return;
            e.preventDefault();
            toggleRail();
        }
        document.addEventListener("keydown", onKey);
        return () => document.removeEventListener("keydown", onKey);
    }, []);

    const iconBtn =
        "grid size-7 shrink-0 place-items-center rounded-md text-subtle-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/40";

    return (
        <motion.aside
            initial={false}
            animate={{ width: rail ? 56 : 244 }}
            transition={{ duration: 0.28, ease: EASE }}
            className="group/sidebar hidden shrink-0 flex-col overflow-hidden md:flex"
        >
            <div className={cn("flex h-[52px] shrink-0 items-center gap-1 pt-1", rail ? "justify-center px-2" : "px-3")}>
                <Link
                    to="/"
                    aria-label="Overview"
                    className={cn(
                        "flex h-8 min-w-0 items-center rounded-md transition-colors outline-none hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-ring/40",
                        rail ? "w-8 justify-center" : "px-1.5",
                    )}
                >
                    {rail ? <SidebarMark /> : <SidebarBrand />}
                </Link>
                {!rail && (
                    <div className="ml-auto flex items-center gap-0.5">
                        <button
                            type="button"
                            onClick={toggleRail}
                            aria-label="Collapse sidebar"
                            title="Collapse sidebar  ["
                            className={cn(iconBtn, "opacity-0 group-hover/sidebar:opacity-100 focus-visible:opacity-100")}
                        >
                            <PanelLeft className="size-4" />
                        </button>
                        <button
                            type="button"
                            onClick={openCommandPalette}
                            aria-label="Search and go to"
                            title={`Search (${IS_MAC ? "⌘" : "Ctrl+"}K)`}
                            className={iconBtn}
                        >
                            <Search className="size-4" />
                        </button>
                    </div>
                )}
            </div>

            {rail && (
                <div className="flex shrink-0 justify-center pb-2">
                    <RailTip label={`Search  ${IS_MAC ? "⌘" : "Ctrl "}K`}>
                        <button type="button" onClick={openCommandPalette} aria-label="Search and go to" className={iconBtn}>
                            <Search className="size-4" />
                        </button>
                    </RailTip>
                </div>
            )}

            <nav className={cn("flex-1 overflow-x-hidden overflow-y-auto pt-1 pb-3", rail ? "px-2" : "px-3")}>
                <NavList rail={rail} />
            </nav>

            <div className={cn("shrink-0 pb-2", rail ? "flex flex-col items-center gap-1.5 px-2" : "px-2")}>
                {rail ? (
                    <RailTip label="Expand sidebar  [">
                        <button type="button" onClick={toggleRail} aria-label="Expand sidebar" className={iconBtn}>
                            <PanelLeft className="size-4" />
                        </button>
                    </RailTip>
                ) : (
                    <div className="flex flex-wrap items-center gap-1.5 px-1 pb-1.5">
                        <EnvPill />
                        <UpdatePill />
                    </div>
                )}
                <div className={cn(!rail && "border-t border-sidebar-border pt-2")}>
                    <UserMenu compact={rail} />
                </div>
            </div>
        </motion.aside>
    );
}

function RailTip({ label, children }: { label: string; children: React.ReactNode }) {
    return (
        <Tooltip delayDuration={150}>
            <TooltipTrigger asChild>{children}</TooltipTrigger>
            <TooltipContent side="right" sideOffset={10}>
                {label}
            </TooltipContent>
        </Tooltip>
    );
}

const COLLAPSED_KEY = "warmbly-admin:nav-collapsed";

function readCollapsed(): Set<string> {
    try {
        const raw = localStorage.getItem(COLLAPSED_KEY);
        return new Set(raw ? (JSON.parse(raw) as string[]) : []);
    } catch {
        return new Set();
    }
}

// NavList renders the permission-filtered groups. Shared by the desktop rail
// and the mobile drawer; onNavigate lets the drawer close itself on a tap.
export function NavList({ onNavigate, rail = false }: { onNavigate?: () => void; rail?: boolean }) {
    const { data: me } = useMe();
    const mask = me?.admin_permissions;
    const canReadHealth = hasAdminPerm(mask, AdminPerm.ViewAnalytics);
    const healthQ = useInstanceHealth({ enabled: canReadHealth });
    const findings = findingCount(healthQ.data);
    const worst = worstSeverity(healthQ.data);
    const [collapsed, setCollapsed] = useState(readCollapsed);
    // One sliding highlight per list, so the drawer and the rail never share it.
    const groupId = useId();

    function toggle(label: string) {
        setCollapsed((prev) => {
            const next = new Set(prev);
            if (next.has(label)) next.delete(label);
            else next.add(label);
            try {
                localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
            } catch {
                /* ignore */
            }
            return next;
        });
    }

    return (
        <LayoutGroup id={groupId}>
            <div className={rail ? "space-y-3" : "space-y-4"}>
                {visibleNavGroups(mask).map((group) => {
                    const titled = group.label !== "Overview";
                    // The rail has no section headers to reopen a section with.
                    const closed = titled && !rail && collapsed.has(group.label);
                    return (
                        <div key={group.label}>
                            {titled &&
                                (rail ? (
                                    <div className="mx-auto mb-2 h-px w-5 bg-sidebar-border" />
                                ) : (
                                    <button
                                        type="button"
                                        onClick={() => toggle(group.label)}
                                        aria-expanded={!closed}
                                        className="group/section mb-0.5 flex h-7 w-full items-center gap-1 rounded-md px-2 text-left text-xs font-medium whitespace-nowrap text-subtle-foreground transition-colors hover:text-sidebar-foreground"
                                    >
                                        {group.label}
                                        <motion.span
                                            initial={false}
                                            animate={{ rotate: closed ? 0 : 90 }}
                                            transition={{ duration: 0.2, ease: EASE }}
                                            className={cn(
                                                "grid place-items-center transition-opacity",
                                                closed ? "opacity-100" : "opacity-0 group-hover/section:opacity-100",
                                            )}
                                        >
                                            <ChevronRight className="size-3" />
                                        </motion.span>
                                    </button>
                                ))}
                            <AnimatePresence initial={false}>
                                {!closed && (
                                    <motion.ul
                                        key="items"
                                        initial={{ height: 0, opacity: 0 }}
                                        animate={{ height: "auto", opacity: 1 }}
                                        exit={{ height: 0, opacity: 0 }}
                                        transition={{ duration: 0.22, ease: EASE }}
                                        className="space-y-px overflow-hidden"
                                    >
                                        {group.items.map((item) => (
                                            <li key={item.label}>
                                                <SidebarLink
                                                    {...item}
                                                    rail={rail}
                                                    onNavigate={onNavigate}
                                                    badge={
                                                        item.healthBadge && findings > 0
                                                            ? { count: findings, severity: worst }
                                                            : undefined
                                                    }
                                                />
                                            </li>
                                        ))}
                                    </motion.ul>
                                )}
                            </AnimatePresence>
                        </div>
                    );
                })}
            </div>
        </LayoutGroup>
    );
}

interface CountBadge {
    count: number;
    severity: CheckSeverity | null;
}

const BADGE_TONES: Record<CheckSeverity, string> = {
    error: "bg-red-500/15 text-red-600 dark:text-red-400",
    warning: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
    info: "bg-sky-500/15 text-sky-700 dark:text-sky-400",
};

const DOT_TONES: Record<CheckSeverity, string> = {
    error: "bg-red-500",
    warning: "bg-amber-500",
    info: "bg-sky-500",
};

function SidebarLink({
    label,
    icon: Icon,
    pages,
    badge,
    rail,
    onNavigate,
}: NavItem & { badge?: CountBadge; rail?: boolean; onNavigate?: () => void }) {
    const { pathname } = useLocation();
    const isActive = sectionActive({ label, icon: Icon, pages }, pathname);
    const link = (
        <Link
            to={pages[0].to}
            onClick={onNavigate}
            aria-label={rail ? label : undefined}
            aria-current={isActive ? "page" : undefined}
            className={cn(
                "group relative flex h-7 items-center gap-2 rounded-md text-[13px] whitespace-nowrap transition-colors duration-150 outline-none focus-visible:ring-2 focus-visible:ring-ring/40",
                rail ? "w-full justify-center px-0" : "px-2",
                isActive
                    ? "font-medium text-sidebar-accent-foreground"
                    : "text-sidebar-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
            )}
        >
            {isActive && (
                <motion.span
                    layoutId="nav-active"
                    transition={SPRING}
                    className="absolute inset-0 rounded-md bg-sidebar-accent shadow-[inset_0_0_0_1px_oklch(1_0_0/0.03)] dark:shadow-[inset_0_1px_0_oklch(1_0_0/0.05)]"
                />
            )}
            <Icon
                className={cn(
                    "relative size-4 shrink-0 transition-colors duration-150",
                    isActive ? "text-sidebar-accent-foreground" : "text-subtle-foreground group-hover:text-sidebar-foreground",
                )}
            />
            {!rail && <span className="relative truncate">{label}</span>}
            {badge &&
                (rail ? (
                    <span
                        className={cn(
                            "absolute top-1 right-1.5 size-1.5 rounded-full ring-2 ring-sidebar",
                            DOT_TONES[badge.severity ?? "info"],
                        )}
                    />
                ) : (
                    <motion.span
                        initial={{ scale: 0.6, opacity: 0 }}
                        animate={{ scale: 1, opacity: 1 }}
                        transition={SPRING}
                        className={cn(
                            "relative ml-auto shrink-0 rounded-[4px] px-1.5 text-[11px] font-medium leading-[18px] tabular-nums",
                            BADGE_TONES[badge.severity ?? "info"],
                        )}
                    >
                        {badge.count}
                    </motion.span>
                ))}
        </Link>
    );
    return rail ? <RailTip label={label}>{link}</RailTip> : link;
}
