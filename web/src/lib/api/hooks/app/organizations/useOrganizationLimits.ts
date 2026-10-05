import { queryOptions, useQuery } from "@tanstack/react-query";
import getOrganizationLimits from "@/lib/api/client/app/organizations/getOrganizationLimits";

export const organizationLimitsQuery = queryOptions({
    queryKey: ["organizations", "limits"],
    queryFn: () => getOrganizationLimits(),
});

export default function useOrganizationLimits() {
    return useQuery(organizationLimitsQuery)
}
