import { queryOptions, useQuery } from "@tanstack/react-query";
import listIntegrationCatalog from "@/lib/api/client/app/integrations/listCatalog";

export const integrationCatalogQuery = queryOptions({
    queryKey: ["integrations", "catalog"],
    queryFn: listIntegrationCatalog,
    // Catalog is effectively static — refresh once an hour at most.
    staleTime: 60 * 60_000,
});

export default function useIntegrationCatalog() {
    return useQuery(integrationCatalogQuery);
}
