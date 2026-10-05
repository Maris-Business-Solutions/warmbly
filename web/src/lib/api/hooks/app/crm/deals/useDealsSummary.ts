import { queryOptions, useQuery } from "@tanstack/react-query";
import type SearchDeals from "@/lib/api/models/app/crm/SearchDeals";
import dealsSummary from "@/lib/api/client/app/crm/deals/dealsSummary";

// Server-aggregated totals for the same filter the table renders. Kept as a
// separate query (not folded into the list) so the header stats and per-stage
// board headers stay correct over the whole set while the rows page in.
export const dealsSummaryQuery = (filters: SearchDeals) =>
    queryOptions({
        queryKey: ["crm", "deals", "summary", filters],
        queryFn: () => dealsSummary(filters),
        staleTime: 30_000,
    });

export default function useDealsSummary(filters: SearchDeals, enabled = true) {
    return useQuery({
        ...dealsSummaryQuery(filters),
        enabled,
    });
}
