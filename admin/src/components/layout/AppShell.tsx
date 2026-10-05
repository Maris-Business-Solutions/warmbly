// Authenticated app shell: the sidebar sits on the base
// surface and the page lives in a raised, rounded panel beside it. The panel
// scrolls on its own, so PageHeader can stick to its top edge. Route guard
// lives in RequireAdmin, which wraps this in main.tsx.

import { Outlet, useLocation } from "react-router-dom";
import { ConfirmProvider } from "@/components/ConfirmDialog";
import { ReauthDialog } from "@/components/ReauthDialog";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { CommandPalette } from "./CommandPalette";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { findSection } from "./nav";

export function AppShell() {
    useDocumentTitle();
    const { pathname } = useLocation();
    const section = findSection(pathname);
    // A section's own pages share a key so its tabs switch instantly; detail pages ease in.
    const pageKey = section?.pages.some((p) => p.to === pathname) ? section.label : pathname;

    return (
        <ConfirmProvider>
            <div className="flex h-dvh overflow-hidden bg-sidebar text-foreground">
                <Sidebar />
                <div className="flex min-w-0 flex-1 flex-col md:py-2 md:pr-2">
                    <div className="relative isolate flex min-h-0 flex-1 flex-col overflow-hidden bg-background md:rounded-xl md:shadow-panel">
                        <Topbar />
                        <main
                            id="main"
                            className="min-h-0 flex-1 overflow-y-auto [--page-x:1rem] [--page-y:1.25rem] md:[--page-x:2rem] md:[--page-y:1.5rem]"
                        >
                            {/* Keyed by section, so moving between sections eases in and tabs switch instantly. */}
                            <div key={pageKey} className="animate-page-in px-[var(--page-x)] pt-[var(--page-y)] pb-16">
                                <Outlet />
                            </div>
                        </main>
                    </div>
                </div>
            </div>
            <CommandPalette />
            <ReauthDialog />
        </ConfirmProvider>
    );
}
