import { useQuery } from "@tanstack/react-query";
import listCrmOwners from "@/lib/api/client/app/crm/provider/listCrmOwners";

export default function useCrmOwners(enabled = true) {
    return useQuery({
        queryKey: ["crm", "owners"],
        queryFn: async () => (await listCrmOwners()).data,
        staleTime: 60_000,
        enabled,
    });
}
