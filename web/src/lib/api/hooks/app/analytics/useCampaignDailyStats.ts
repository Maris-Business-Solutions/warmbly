import { keepPreviousData, useQuery } from "@tanstack/react-query";
import getCampaignDailyStats from "@/lib/api/client/app/analytics/getCampaignDailyStats";

// One row per UTC day of the window with sends; waits while the window is
// null. The realtime invalidation key ["analytics","campaigns",id,"daily"]
// still matches this (prefix) so live events refresh the chart.
export default function useCampaignDailyStats(id: string, window: { from: string; to: string } | null) {
    return useQuery({
        queryKey: ["analytics", "campaigns", id, "daily", window?.from, window?.to],
        queryFn: () => getCampaignDailyStats(id, window!.from, window!.to),
        enabled: !!id && !!window,
        placeholderData: keepPreviousData,
    });
}
