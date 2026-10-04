import useCrmSettings from "@/lib/api/hooks/app/crm/provider/useCrmSettings";
import type { CRMSettings } from "@/lib/api/models/app/crm/CRMProvider";

export interface CrmProviderState {
    // True while the workspace runs its CRM on HubSpot.
    isHubSpot: boolean;
    settings?: CRMSettings;
    // HubSpot's web app root for this portal ("" when not connected).
    appUrl: string;
    // The connection lacks a permission CRM mode needs, or was revoked.
    needsReconnect: boolean;
    loading: boolean;
}

// Which CRM the workspace runs on. Every CRM surface reads this to show
// HubSpot's records, wording and logo instead of Warmbly's own.
export default function useCrmProvider(): CrmProviderState {
    const { data, isLoading } = useCrmSettings();
    const isHubSpot = data?.provider === "hubspot";
    const account = data?.account;
    return {
        isHubSpot,
        settings: data,
        appUrl: account?.app_url ?? "",
        needsReconnect:
            isHubSpot &&
            !!account &&
            ((account.missing_scopes?.length ?? 0) > 0 || account.status === "reauth_required" || account.status === "disconnected"),
        loading: isLoading,
    };
}
