import { queryOptions, useQuery } from "@tanstack/react-query";
import listIntegrationConnections from "@/lib/api/client/app/integrations/listConnections";

export const integrationConnectionsQuery = queryOptions({
    queryKey: ["integrations", "connections"],
    queryFn: listIntegrationConnections,
    staleTime: 10_000,
});

export default function useIntegrationConnections() {
    return useQuery(integrationConnectionsQuery);
}
