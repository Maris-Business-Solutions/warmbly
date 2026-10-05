// Mobile navigation drawer. Below `md` the sidebar is hidden, so the
// mobile bar's hamburger opens this left sheet with the same nav model.
// Closes on a tap (onNavigate), on Escape and on the backdrop (Radix), and
// on any route change so a browser back button never leaves it open.

import { useEffect, useRef } from "react";
import { useLocation } from "react-router-dom";
import { Sheet, SheetContent, SheetDescription, SheetTitle } from "@/components/ui/sheet";
import { SidebarBrand } from "./Brand";
import { EnvPill } from "./EnvPill";
import { NavList } from "./Sidebar";
import { UserMenu } from "./UserMenu";

interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function MobileNav({ open, onOpenChange }: Props) {
    const location = useLocation();
    const routeKey = location.pathname + location.search;
    const lastRoute = useRef(routeKey);

    useEffect(() => {
        if (lastRoute.current === routeKey) return;
        lastRoute.current = routeKey;
        onOpenChange(false);
    }, [routeKey, onOpenChange]);

    // The sheet body is hidden at md but its overlay is not, so a resize to
    // desktop with the drawer open would leave an invisible click shield.
    useEffect(() => {
        const media = window.matchMedia("(min-width: 768px)");
        const closeOnDesktop = () => {
            if (media.matches) onOpenChange(false);
        };
        closeOnDesktop();
        media.addEventListener("change", closeOnDesktop);
        return () => media.removeEventListener("change", closeOnDesktop);
    }, [onOpenChange]);

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent
                side="left"
                showCloseButton
                className="w-72 max-w-[85vw] gap-0 bg-sidebar p-0 md:hidden"
            >
                <SheetTitle className="sr-only">Navigation</SheetTitle>
                <SheetDescription className="sr-only">Admin panel sections</SheetDescription>
                <div className="flex h-12 shrink-0 items-center px-4 pr-12">
                    <SidebarBrand />
                </div>
                <nav className="flex-1 overflow-y-auto px-3 pb-3">
                    <NavList onNavigate={() => onOpenChange(false)} />
                </nav>
                <div className="shrink-0 px-2 pb-2">
                    <div className="px-1 pb-1.5">
                        <EnvPill />
                    </div>
                    <div className="border-t border-sidebar-border pt-2">
                        <UserMenu />
                    </div>
                </div>
            </SheetContent>
        </Sheet>
    );
}
