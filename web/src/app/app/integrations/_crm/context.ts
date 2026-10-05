// The CRM a provider page is about (HubSpot or Pipedrive). The page sets it,
// so its wizard and settings read the right words even while the workspace
// still runs on another CRM.

import React from "react";
import { CRM_INFO, type CrmInfo } from "@/components/app/crm/crmProviders";

export const CrmPageContext = React.createContext<CrmInfo>(CRM_INFO.hubspot);

export function usePageCrm(): CrmInfo {
    return React.useContext(CrmPageContext);
}
