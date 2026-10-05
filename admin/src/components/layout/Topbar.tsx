// Mobile-only bar at the top of the page panel: the hamburger, the brand and
// the search button. On desktop the sidebar carries all three.

import { useState } from "react";
import { Menu, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Logo } from "@/components/Logo";
import { openCommandPalette } from "./CommandPalette";
import { MobileNav } from "./MobileNav";

export function Topbar() {
    const [mobileOpen, setMobileOpen] = useState(false);

    return (
        <header className="flex h-12 shrink-0 items-center gap-2 border-b border-border px-2 md:hidden">
            <Button
                variant="ghost"
                size="icon-sm"
                aria-label="Open navigation"
                onClick={() => setMobileOpen(true)}
            >
                <Menu className="size-4" />
            </Button>
            <MobileNav open={mobileOpen} onOpenChange={setMobileOpen} />
            <div className="flex items-center gap-2 text-[13px] font-medium">
                <Logo className="size-4" />
                Warmbly Admin
            </div>
            <Button
                variant="ghost"
                size="icon-sm"
                className="ml-auto"
                aria-label="Search and go to"
                onClick={openCommandPalette}
            >
                <Search className="size-4" />
            </Button>
        </header>
    );
}
