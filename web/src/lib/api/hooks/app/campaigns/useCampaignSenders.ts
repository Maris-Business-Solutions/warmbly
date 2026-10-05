import { queryOptions, useQuery } from "@tanstack/react-query";
import getCampaignSenders from "@/lib/api/client/app/campaigns/getCampaignSenders";

export const campaignSendersQuery = (campaignId: string) =>
    queryOptions({
        queryKey: ["campaigns", campaignId, "senders"],
        queryFn: () => getCampaignSenders(campaignId),
    });

export default function useCampaignSenders(campaignId: string, enabled: boolean = true) {
    return useQuery({
        ...campaignSendersQuery(campaignId),
        enabled: enabled && !!campaignId,
    });
}
