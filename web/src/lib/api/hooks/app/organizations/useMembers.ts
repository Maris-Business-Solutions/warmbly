import { queryOptions, useQuery } from "@tanstack/react-query";
import getMembers from "@/lib/api/client/app/organizations/getMembers";

export const membersQuery = queryOptions({
    queryKey: ["organizations", "members"],
    queryFn: () => getMembers(),
});

export default function useMembers() {
    return useQuery(membersQuery)
}
