import type { LeadCC } from "@/lib/api/models/app/contacts/Contact";
import Request from "../../Request";

export interface LeadCCResult {
    campaign_id: string;
    contact_id: string;
    cc: LeadCC[];
}

// A likely colleague of the lead: same company name, or the same business
// email domain.
export interface LeadCCSuggestion {
    contact_id: string;
    email: string;
    first_name: string;
    last_name: string;
    company?: string;
    reason: "company" | "domain";
}

// Replaces the whole list; an empty list removes every copy.
export async function setCampaignLeadCC(
    campaignId: string,
    contactId: string,
    contactIds: string[],
): Promise<LeadCCResult> {
    return await Request<LeadCCResult>({
        method: "PUT",
        url: `/campaigns/${campaignId}/leads/${contactId}/cc`,
        data: { contact_ids: contactIds },
        authorization: true,
    });
}

export async function getCampaignLeadCCSuggestions(
    campaignId: string,
    contactId: string,
): Promise<{ data: LeadCCSuggestion[] }> {
    return await Request<{ data: LeadCCSuggestion[] }>({
        method: "GET",
        url: `/campaigns/${campaignId}/leads/${contactId}/cc/suggestions`,
        authorization: true,
    });
}
