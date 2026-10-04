import { queryOptions, useQuery } from "@tanstack/react-query";
import getTrialStatus from "@/lib/api/client/app/subscription/getTrialStatus";

export const trialStatusQuery = queryOptions({
    queryKey: ["subscription", "trial"],
    queryFn: () => getTrialStatus(),
});

export default function useTrialStatus() {
    return useQuery(trialStatusQuery)
}
