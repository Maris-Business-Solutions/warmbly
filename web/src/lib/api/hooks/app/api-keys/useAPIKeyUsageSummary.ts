import { queryOptions, useQuery } from "@tanstack/react-query";
import getAPIKeyUsageSummary from "@/lib/api/client/app/api-keys/getAPIKeyUsageSummary";

export const apiKeyUsageSummaryQuery = queryOptions({
    queryKey: ["api-keys", "usage-summary"],
    queryFn: () => getAPIKeyUsageSummary(),
});

export default function useAPIKeyUsageSummary(enabled = true) {
    return useQuery({
        ...apiKeyUsageSummaryQuery,
        enabled,
        refetchInterval: 30_000,
    });
}
