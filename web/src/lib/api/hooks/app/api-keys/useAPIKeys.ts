import { queryOptions, useQuery } from "@tanstack/react-query";
import listAPIKeys from "@/lib/api/client/app/api-keys/listAPIKeys";

export const apiKeysListQuery = queryOptions({
    queryKey: ["api-keys", "list"],
    queryFn: () => listAPIKeys({ limit: 100 }),
    staleTime: 5_000,
});

export default function useAPIKeys() {
    return useQuery(apiKeysListQuery);
}
