import { keepPreviousData, useQuery } from "@tanstack/react-query";
import getCampaignAnalytics from "@/lib/api/client/app/analytics/getCampaignAnalytics";

// A null window is all time. The key stays under ["analytics","campaigns",id]
// so realtime invalidation of the campaign refreshes every window.
export default function useCampaignAnalytics(id: string, window: { from: string; to: string } | null = null) {
    return useQuery({
        queryKey: ["analytics", "campaigns", id, window?.from ?? "all", window?.to ?? "all"],
        queryFn: () => getCampaignAnalytics(id, window),
        enabled: !!id,
        placeholderData: keepPreviousData,
    })
}
