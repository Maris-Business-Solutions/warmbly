import { useQuery } from "@tanstack/react-query";
import getCrmMetadata from "@/lib/api/client/app/crm/provider/getCrmMetadata";

// HubSpot's lifecycle stages, lead statuses, task types, properties and pipelines.
export default function useCrmMetadata(enabled = true) {
    return useQuery({
        queryKey: ["crm", "metadata"],
        queryFn: getCrmMetadata,
        staleTime: 5 * 60_000,
        enabled,
    });
}
