// The signed-in account at the foot of the sidebar. The row shows who is
// signed in; its menu holds notifications, configuration, the theme, a
// shortcut to the dashboard and log out. Logout clears only the admin token
// (dashboard tokens live under a different key) and bounces to /auth/login.

import { Link, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
    BellRing,
    ChevronsUpDown,
    ExternalLink,
    LogOut,
    Monitor,
    Moon,
    SlidersHorizontal,
    Sun,
} from "lucide-react";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuSeparator,
    DropdownMenuSub,
    DropdownMenuSubContent,
    DropdownMenuSubTrigger,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useMe } from "@/hooks/useMe";
import { logout as logoutCall } from "@/lib/api/client/auth";
import { clearToken } from "@/lib/auth/storage";
import { DASHBOARD_URL } from "@/lib/env";
import { useTheme, type ThemePref } from "@/lib/theme";
import { cn } from "@/lib/utils";

function Avatar({ initials, className }: { initials: string; className?: string }) {
    return (
        <span
            className={cn(
                "grid size-7 shrink-0 place-items-center rounded-full bg-sidebar-accent text-[11px] font-semibold text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]",
                className,
            )}
        >
            {initials}
        </span>
    );
}

export function UserMenu({ compact = false }: { compact?: boolean }) {
    const { data: me } = useMe();
    const nav = useNavigate();
    const qc = useQueryClient();
    const theme = useTheme();
    const canConfigure = useAdminPerm(AdminPerm.ManageSettings);

    async function handleLogout() {
        try {
            await logoutCall();
        } catch {
            /* even if the server call fails, drop our local session */
        } finally {
            clearToken();
            qc.clear();
            nav("/auth/login", { replace: true });
        }
    }

    const name = [me?.first_name, me?.last_name].filter(Boolean).join(" ").trim();
    const initials =
        ((me?.first_name?.[0] ?? "") + (me?.last_name?.[0] ?? "")).toUpperCase() ||
        (me?.email?.[0]?.toUpperCase() ?? "?");
    const ThemeIcon = theme.resolved === "dark" ? Moon : Sun;

    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                {compact ? (
                    <button
                        type="button"
                        aria-label={`Signed in as ${me?.email ?? ""}`}
                        className="grid size-9 place-items-center rounded-md transition-colors outline-none hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-ring/40 data-[state=open]:bg-sidebar-accent"
                    >
                        <Avatar initials={initials} />
                    </button>
                ) : (
                    <button
                        type="button"
                        className="flex h-11 w-full min-w-0 items-center gap-2.5 rounded-lg px-2 text-left transition-colors outline-none hover:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-ring/40 data-[state=open]:bg-sidebar-accent"
                    >
                        <Avatar initials={initials} />
                        <span className="min-w-0 flex-1 leading-tight">
                            <span className="block truncate text-[13px] font-medium text-sidebar-accent-foreground">
                                {name || me?.email || "Signed in"}
                            </span>
                            {name && <span className="block truncate text-[11.5px] text-subtle-foreground">{me?.email}</span>}
                        </span>
                        <ChevronsUpDown className="size-3.5 shrink-0 text-subtle-foreground" />
                    </button>
                )}
            </DropdownMenuTrigger>
            <DropdownMenuContent side="top" align="start" sideOffset={6} className="w-[var(--radix-dropdown-menu-trigger-width)] min-w-60">
                <DropdownMenuLabel className="flex items-center gap-2.5 py-2 font-normal">
                    <Avatar initials={initials} className="size-8 bg-secondary" />
                    <span className="min-w-0">
                        <span className="block text-[11.5px] text-muted-foreground">Signed in as</span>
                        <span className="block truncate text-[13px] text-foreground">{me?.email ?? "Unknown"}</span>
                    </span>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                {canConfigure && (
                    <>
                        <DropdownMenuItem asChild>
                            <Link to="/configuration">
                                <SlidersHorizontal />
                                Configuration
                            </Link>
                        </DropdownMenuItem>
                        <DropdownMenuItem asChild>
                            <Link to="/configuration?tab=notifications">
                                <BellRing />
                                Notifications
                            </Link>
                        </DropdownMenuItem>
                    </>
                )}
                <DropdownMenuSub>
                    <DropdownMenuSubTrigger>
                        <ThemeIcon />
                        Theme
                    </DropdownMenuSubTrigger>
                    <DropdownMenuSubContent className="w-40">
                        <DropdownMenuRadioGroup value={theme.pref} onValueChange={(v) => theme.setPref(v as ThemePref)}>
                            <DropdownMenuRadioItem value="dark">
                                <Moon className="size-3.5 text-muted-foreground" />
                                Dark
                            </DropdownMenuRadioItem>
                            <DropdownMenuRadioItem value="light">
                                <Sun className="size-3.5 text-muted-foreground" />
                                Light
                            </DropdownMenuRadioItem>
                            <DropdownMenuRadioItem value="system">
                                <Monitor className="size-3.5 text-muted-foreground" />
                                System
                            </DropdownMenuRadioItem>
                        </DropdownMenuRadioGroup>
                    </DropdownMenuSubContent>
                </DropdownMenuSub>
                <DropdownMenuItem asChild>
                    <a href={DASHBOARD_URL} target="_blank" rel="noreferrer">
                        <ExternalLink />
                        Open dashboard
                    </a>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={handleLogout} variant="destructive">
                    <LogOut />
                    Log out
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    );
}
