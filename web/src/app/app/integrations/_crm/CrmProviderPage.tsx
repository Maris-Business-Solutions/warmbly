// A connected CRM's home (HubSpot or Pipedrive). Not connected: what its mode
// does and a Connect button. Connected but not the CRM yet (or setup
// unfinished): the setup wizard. Its mode: health and every setting, editable.

import type { ReactNode } from "react";
import { Loader2Icon } from "lucide-react";

import { CRM_INFO, type ExternalCrm } from "@/components/app/crm/crmProviders";
import { Page } from "@/components/layout/Page";
import useCrmSettings from "@/lib/api/hooks/app/crm/provider/useCrmSettings";
import useIntegrationCatalog from "@/lib/api/hooks/app/integrations/useIntegrationCatalog";
import useIntegrationConnections from "@/lib/api/hooks/app/integrations/useIntegrationConnections";

import { CrmPageContext } from "./context";
import CrmSettings from "./CrmSettings";
import Hero from "./Hero";
import SetupWizard from "./SetupWizard";
import { errMessage } from "./shared";

export default function CrmProviderPage({ provider }: { provider: ExternalCrm }) {
    const crm = CRM_INFO[provider];
    const settings = useCrmSettings();
    const connections = useIntegrationConnections();
    const catalog = useIntegrationCatalog();

    const mine = (connections.data?.connections ?? []).filter((c) => c.provider === provider);
    const s = settings.data;

    let body: ReactNode;
    if (settings.isLoading || connections.isLoading) {
        body = (
            <div className="flex-1 flex items-center justify-center py-24 text-[12px] text-slate-400 gap-2">
                <Loader2Icon className="w-3.5 h-3.5 animate-spin" />
                Loading {crm.name}…
            </div>
        );
    } else if (settings.isError || !s) {
        body = (
            <div className="max-w-xl mx-auto px-5 py-16 text-center">
                <p className="text-[13px] font-medium text-slate-900">{crm.name} is not available here</p>
                <p className="text-[12px] text-slate-500 mt-1">{errMessage(settings.error, "Could not load your CRM settings.")}</p>
            </div>
        );
    } else if (s.provider === provider && s.setup_completed_at) {
        body = (
            <CrmSettings
                settings={s}
                connection={mine.find((c) => c.id === s.connection_id)}
                entry={catalog.data?.catalog.find((e) => e.provider === provider)}
            />
        );
    } else if (mine.length > 0 || s.provider === provider) {
        // Keyed by provider so a page switch never reuses the other's draft.
        body = <SetupWizard key={provider} settings={s} connections={mine} />;
    } else {
        body = <Hero />;
    }

    return (
        <CrmPageContext.Provider value={crm}>
            <Page>{body}</Page>
        </CrmPageContext.Provider>
    );
}
