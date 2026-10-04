import getCampaign from "@/lib/api/client/app/campaigns/getCampaign";
import { queryOptions, useQuery } from "@tanstack/react-query";

export const campaignQuery = (id: string) =>
    queryOptions({
        queryKey: ["campaigns", id],
        queryFn: () => getCampaign(id),
    });

// Plain useQuery (not useSuspenseQuery) so the layout can gate on
// isLoading/isError. The detail route has no Suspense boundary, and a
// suspense query there throws a promise on every render with nothing to
// catch it — the page spins forever. useQuery resolves to a real loading
// flag the layout can branch on.
const useCampaign = (id: string) =>
    useQuery({
        ...campaignQuery(id),
        enabled: !!id,
    });

export default useCampaign;
