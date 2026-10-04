import { useQuery } from "@tanstack/react-query";
import getCrmSyncHealth from "@/lib/api/client/app/crm/provider/getCrmSyncHealth";

// Outbox counts, failures and pull freshness. CRM_SYNCED keeps it live.
export default function useCrmSyncHealth(enabled = true) {
    return useQuery({
        queryKey: ["crm", "sync"],
        queryFn: getCrmSyncHealth,
        staleTime: 15_000,
        enabled,
    });
}
