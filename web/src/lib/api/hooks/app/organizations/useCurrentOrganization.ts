import { queryOptions, useQuery } from "@tanstack/react-query";
import getCurrentOrganization from "@/lib/api/client/app/organizations/getCurrentOrganization";

export const currentOrganizationQuery = queryOptions({
    queryKey: ["organizations", "current"],
    queryFn: () => getCurrentOrganization(),
    staleTime: 60_000,
});

export default function useCurrentOrganization() {
    return useQuery({
        ...currentOrganizationQuery,
        refetchOnMount: false,
    });
}
