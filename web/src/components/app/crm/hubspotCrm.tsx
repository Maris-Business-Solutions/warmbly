// Shared HubSpot-mode pieces for the CRM screens: the header chip that says
// where the records live, the owner-mapping hint and the automation note.

import React from "react";
import { Link } from "@tanstack/react-router";
import { AlertTriangleIcon } from "lucide-react";
import useCrmProvider from "@/hooks/useCrmProvider";
import useCrmSyncHealth from "@/lib/api/hooks/app/crm/provider/useCrmSyncHealth";
import { cn } from "@/lib/utils";
import { HubSpotBadge, HubSpotMark, HubSpotSyncedAt } from "./HubSpot";

export const HUBSPOT_SETTINGS_PATH = "/app/integrations/hubspot";

// Topbar chip for a CRM screen in HubSpot mode: the last sync, or a reconnect
// link when the connection is broken. Renders nothing in native mode.
export function HubSpotHeaderStatus({ syncedAt, className }: { syncedAt?: Date | string; className?: string }) {
    const { isHubSpot, needsReconnect } = useCrmProvider();
    const health = useCrmSyncHealth(isHubSpot && !syncedAt);
    if (!isHubSpot) return null;
    if (needsReconnect) {
        return (
            <Link
                to={HUBSPOT_SETTINGS_PATH}
                className={cn(
                    "inline-flex items-center gap-1 h-6 px-2 rounded-md bg-amber-50 border border-amber-200 text-[11px] text-amber-800 hover:bg-amber-100 transition-colors shrink-0",
                    className,
                )}
            >
                <AlertTriangleIcon className="w-3 h-3" />
                Reconnect HubSpot
            </Link>
        );
    }
    const at = syncedAt ?? health.data?.last_synced_at;
    return (
        <span className={cn("inline-flex items-center gap-2 shrink-0", className)}>
            {at ? <HubSpotSyncedAt at={at} className="hidden sm:inline-flex" /> : null}
            <HubSpotBadge className={at ? "sm:hidden" : undefined} />
        </span>
    );
}

// A one-line hint pointing at the owner mapping, for pickers that disable
// members HubSpot does not know.
export function HubSpotOwnerMappingLink({ onNavigate, className }: { onNavigate?: () => void; className?: string }) {
    return (
        <Link
            to={HUBSPOT_SETTINGS_PATH}
            onClick={onNavigate}
            className={cn(
                "block px-3 py-1.5 text-[11px] leading-snug text-slate-500 hover:text-orange-700 transition-colors",
                className,
            )}
        >
            Members marked "Not in HubSpot" need a HubSpot owner. Match them in HubSpot settings.
        </Link>
    );
}

// An inline note on an automation or sequence step that writes to HubSpot.
// Renders nothing outside HubSpot mode.
export function HubSpotActionNote({ children, className }: { children: React.ReactNode; className?: string }) {
    const { isHubSpot } = useCrmProvider();
    if (!isHubSpot) return null;
    return (
        <p
            className={cn(
                "flex items-start gap-1.5 rounded-md border border-orange-200 bg-[#FF7A59]/[0.06] px-2.5 py-2 text-[11px] leading-relaxed text-slate-600",
                className,
            )}
        >
            <HubSpotMark className="mt-px w-3.5 h-3.5" />
            <span className="min-w-0">{children}</span>
        </p>
    );
}
