import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
    getCampaignLeadCCSuggestions,
    setCampaignLeadCC,
    type LeadCCResult,
} from "@/lib/api/client/app/campaigns/leadCC";

// Changing who a lead copies moves the Leads list (its CC badge, and a copied
// contact's own lead being held or released) and the contact drawers. The
// server audits it, so teammates get it over the audit spine; this is the
// local echo.
export function useSetLeadCC() {
    const queryClient = useQueryClient();
    return useMutation<LeadCCResult, unknown, { campaignId: string; contactId: string; contactIds: string[] }>({
        mutationFn: ({ campaignId, contactId, contactIds }) => setCampaignLeadCC(campaignId, contactId, contactIds),
        onSuccess: () => void queryClient.invalidateQueries({ queryKey: ["contacts"] }),
    });
}

// Keyed under ["contacts", id] so the same invalidations refresh it.
export function useLeadCCSuggestions(campaignId: string, contactId: string, enabled: boolean) {
    return useQuery({
        queryKey: ["contacts", contactId, "cc-suggestions", campaignId],
        queryFn: () => getCampaignLeadCCSuggestions(campaignId, contactId),
        enabled: enabled && !!campaignId && !!contactId,
        staleTime: 30_000,
    });
}
