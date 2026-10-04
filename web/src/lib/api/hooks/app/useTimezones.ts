import { queryOptions, useQuery } from "@tanstack/react-query";
import getTimezones from "../../client/app/getTimezones";

// Timezones are static reference data. Fetch once per session, never
// refetch — saves a round trip on every page mount in the bootstrap
// chain.
export const timezonesQuery = queryOptions({
    queryKey: ["timezones"],
    queryFn: () => getTimezones(),
    staleTime: Infinity,
    gcTime: Infinity,
    refetchOnMount: false,
});

export default function useTimezones() {
    return useQuery(timezonesQuery);
}
