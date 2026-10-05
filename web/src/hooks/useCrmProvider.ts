import { type CrmInfo, crmInfo } from "@/components/app/crm/crmProviders";
import useCrmSettings from "@/lib/api/hooks/app/crm/provider/useCrmSettings";
import type { CRMProviderName, CRMSettings } from "@/lib/api/models/app/crm/CRMProvider";

export interface CrmProviderState {
    provider: CRMProviderName;
    // True while the workspace runs its CRM on a connected one (HubSpot, Pipedrive).
    isExternal: boolean;
    // The connected CRM's name, logo and words; HubSpot's while none is active.
    crm: CrmInfo;
    settings?: CRMSettings;
    // The provider's web app root for this account ("" when not connected).
    appUrl: string;
    // The connection lacks a permission CRM mode needs, or was revoked.
    needsReconnect: boolean;
    loading: boolean;
}

// Which CRM the workspace runs on. Every CRM surface reads this to show the
// connected CRM's records, wording and logo instead of Warmbly's own.
export default function useCrmProvider(): CrmProviderState {
    const { data, isLoading } = useCrmSettings();
    const provider = data?.provider ?? "native";
    const isExternal = provider === "hubspot" || provider === "pipedrive";
    const account = data?.account;
    return {
        provider,
        isExternal,
        crm: crmInfo(provider),
        settings: data,
        appUrl: account?.app_url ?? "",
        needsReconnect:
            isExternal &&
            !!account &&
            ((account.missing_scopes?.length ?? 0) > 0 || account.status === "reauth_required" || account.status === "disconnected"),
        loading: isLoading,
    };
}
