import getSequences from "@/lib/api/client/app/campaigns/sequences/getSequences";
import { queryOptions, useSuspenseQuery } from "@tanstack/react-query";

export const sequencesQuery = (campaign_id: string) => queryOptions({
    queryKey: ["campaigns", campaign_id, "sequences"],
    queryFn: () => getSequences(campaign_id),
})

const useSequences = (campaign_id: string) => useSuspenseQuery(sequencesQuery(campaign_id))

export default useSequences;
