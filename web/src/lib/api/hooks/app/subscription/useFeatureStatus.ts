import { queryOptions, useQuery } from "@tanstack/react-query";
import getFeatureStatus from "@/lib/api/client/app/subscription/getFeatureStatus";

export const featureStatusQuery = queryOptions({
    queryKey: ["subscription", "features"],
    queryFn: () => getFeatureStatus(),
});

export default function useFeatureStatus() {
    return useQuery(featureStatusQuery)
}
