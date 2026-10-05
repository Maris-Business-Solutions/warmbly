import { queryOptions, useQuery } from "@tanstack/react-query";
import listTeams from "@/lib/api/client/app/teams/listTeams";

export const teamsQuery = queryOptions({
    queryKey: ["teams"],
    queryFn: () => listTeams(),
    staleTime: 5 * 60 * 1000,
});

export default function useTeams() {
    return useQuery(teamsQuery);
}
