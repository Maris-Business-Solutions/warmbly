import { queryOptions, useQuery } from "@tanstack/react-query";
import getAPIKeyAnalytics, { type AnalyticsParams } from "@/lib/api/client/app/api-keys/getAPIKeyAnalytics";

export const apiKeyAnalyticsQuery = (keyID: string | "all", params?: AnalyticsParams) =>
    queryOptions({
        queryKey: ["api-keys", "analytics", keyID, params],
        queryFn: () => getAPIKeyAnalytics(keyID, params),
    });

export default function useAPIKeyAnalytics(keyID: string | "all", params?: AnalyticsParams) {
    return useQuery({
        ...apiKeyAnalyticsQuery(keyID, params),
        refetchInterval: 60_000,
    });
}
