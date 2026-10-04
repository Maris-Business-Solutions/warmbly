// Shared provider-mode pieces for the CRM screens: the header chip that says
// where the records live, the owner-mapping hint and the automation note.

import React from "react";
import { Link } from "react-router-dom";
import { AlertTriangleIcon } from "lucide-react";
import useCrmProvider from "@/hooks/useCrmProvider";
import useCrmSyncHealth from "@/lib/api/hooks/app/crm/provider/useCrmSyncHealth";
import { cn } from "@/lib/utils";
import { CrmBadge, CrmMark, CrmSyncedAt } from "./crmProviders";

// Topbar chip for a CRM screen in provider mode: the last sync, or a reconnect
// link when the connection is broken. Renders nothing in native mode.
export function CrmHeaderStatus({ syncedAt, className }: { syncedAt?: Date | string; className?: string }) {
    const { isExternal, crm, needsReconnect } = useCrmProvider();
    const health = useCrmSyncHealth(isExternal && !syncedAt);
    if (!isExternal) return null;
    if (needsReconnect) {
        return (
            <Link
                to={crm.settingsPath}
                className={cn(
                    "inline-flex items-center gap-1 h-6 px-2 rounded-md bg-amber-50 border border-amber-200 text-[11px] text-amber-800 hover:bg-amber-100 transition-colors shrink-0",
                    className,
                )}
            >
                <AlertTriangleIcon className="w-3 h-3" />
                Reconnect {crm.name}
            </Link>
        );
    }
    const at = syncedAt ?? health.data?.last_synced_at;
    return (
        <span className={cn("inline-flex items-center gap-2 shrink-0", className)}>
            {at ? <CrmSyncedAt at={at} provider={crm.id} className="hidden sm:inline-flex" /> : null}
            <CrmBadge provider={crm.id} className={at ? "sm:hidden" : undefined} />
        </span>
    );
}

// A one-line hint pointing at the owner mapping, for pickers that disable
// members the CRM does not know.
export function CrmOwnerMappingLink({ onNavigate, className }: { onNavigate?: () => void; className?: string }) {
    const { crm } = useCrmProvider();
    return (
        <Link
            to={crm.settingsPath}
            onClick={onNavigate}
            className={cn("block px-3 py-1.5 text-[11px] leading-snug text-slate-500 transition-colors", crm.hoverText, className)}
        >
            Members marked "Not in {crm.name}" need a {crm.name} {crm.words.owner}. Match them in {crm.name} settings.
        </Link>
    );
}

// An inline note on an automation or sequence step that writes to the
// connected CRM. Renders nothing in native mode.
export function CrmActionNote({ children, className }: { children: React.ReactNode; className?: string }) {
    const { isExternal, crm } = useCrmProvider();
    if (!isExternal) return null;
    return (
        <p
            className={cn(
                "flex items-start gap-1.5 rounded-md border px-2.5 py-2 text-[11px] leading-relaxed text-slate-600",
                crm.border,
                crm.tint,
                className,
            )}
        >
            <CrmMark provider={crm.id} className="mt-px w-3.5 h-3.5" />
            <span className="min-w-0">{children}</span>
        </p>
    );
}

// The legacy upsert step of the CRM the workspace runs on: the sync does its
// job now. Renders nothing for any other step or mode.
export function CrmUpsertNote({ action }: { action: string }) {
    const { isExternal, crm } = useCrmProvider();
    const own =
        (crm.id === "hubspot" && action === "hubspot.upsert_contact") ||
        (crm.id === "pipedrive" && action === "pipedrive.upsert_person");
    if (!isExternal || !own) return null;
    return (
        <CrmActionNote>
            {crm.name} sync is automatic now: {crm.words.contacts} are created and updated in {crm.name} as they move
            through Warmbly, so this step is no longer needed. It keeps running if you leave it.
        </CrmActionNote>
    );
}

// A deal step in provider mode: where the deal is created or moved.
export function CrmDealNote({ creates, className }: { creates: boolean; className?: string }) {
    const { crm } = useCrmProvider();
    return (
        <CrmActionNote className={className}>
            {creates
                ? `The deal is created in ${crm.name}, in the pipeline and deal stage you pick.`
                : `The deal moves in ${crm.name} too. Pipelines and deal stages are ${crm.name}'s.`}
        </CrmActionNote>
    );
}

// A task step in provider mode: what the task becomes there.
export function CrmTaskNote({ className }: { className?: string }) {
    const { crm } = useCrmProvider();
    return (
        <CrmActionNote className={className}>
            {crm.id === "pipedrive"
                ? "The task is created in Pipedrive as an activity of the type you pick. A member needs a Pipedrive user to be assigned."
                : "The task is created in HubSpot with a HubSpot task type. A member needs a HubSpot owner to be assigned."}
        </CrmActionNote>
    );
}
