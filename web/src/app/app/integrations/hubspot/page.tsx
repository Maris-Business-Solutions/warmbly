// The HubSpot home. Not connected: what HubSpot mode does and a Connect button.
// Connected but not the CRM yet (or setup unfinished): the setup wizard.
// HubSpot mode: health and every setting, editable.

"use client";

import type { ReactNode } from "react";
import { Loader2Icon } from "lucide-react";

import { Page } from "@/components/layout/Page";
import useCrmSettings from "@/lib/api/hooks/app/crm/provider/useCrmSettings";
import useIntegrationCatalog from "@/lib/api/hooks/app/integrations/useIntegrationCatalog";
import useIntegrationConnections from "@/lib/api/hooks/app/integrations/useIntegrationConnections";

import Hero from "./_components/Hero";
import HubSpotSettings from "./_components/HubSpotSettings";
import SetupWizard from "./_components/SetupWizard";
import { errMessage } from "./_components/shared";

export default function HubSpotPage() {
    const settings = useCrmSettings();
    const connections = useIntegrationConnections();
    const catalog = useIntegrationCatalog();

    const hubspot = (connections.data?.connections ?? []).filter((c) => c.provider === "hubspot");
    const s = settings.data;

    let body: ReactNode;
    if (settings.isLoading || connections.isLoading) {
        body = (
            <div className="flex-1 flex items-center justify-center py-24 text-[12px] text-slate-400 gap-2">
                <Loader2Icon className="w-3.5 h-3.5 animate-spin" />
                Loading HubSpot…
            </div>
        );
    } else if (settings.isError || !s) {
        body = (
            <div className="max-w-xl mx-auto px-5 py-16 text-center">
                <p className="text-[13px] font-medium text-slate-900">HubSpot is not available here</p>
                <p className="text-[12px] text-slate-500 mt-1">{errMessage(settings.error, "Could not load your CRM settings.")}</p>
            </div>
        );
    } else if (s.provider === "hubspot" && s.setup_completed_at) {
        body = (
            <HubSpotSettings
                settings={s}
                connection={hubspot.find((c) => c.id === s.connection_id)}
                entry={catalog.data?.catalog.find((e) => e.provider === "hubspot")}
            />
        );
    } else if (hubspot.length > 0 || s.provider === "hubspot") {
        body = <SetupWizard settings={s} connections={hubspot} />;
    } else {
        body = <Hero />;
    }

    return <Page>{body}</Page>;
}
