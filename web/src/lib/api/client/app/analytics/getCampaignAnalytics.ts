import type CampaignAnalytics from "@/lib/api/models/app/analytics/CampaignAnalytics";
import Request from "../../Request";

// Without a window the figures cover every send; with one, only the emails
// sent on those UTC days (both "yyyy-MM-dd", to included).
export default async function getCampaignAnalytics(id: string, window?: { from: string; to: string } | null): Promise<CampaignAnalytics> {
    const query = window ? `?${new URLSearchParams({ from: window.from, to: window.to }).toString()}` : "";
    return await Request<CampaignAnalytics>({
        method: "GET",
        url: `/analytics/campaigns/${id}${query}`,
        authorization: true,
    })
}
