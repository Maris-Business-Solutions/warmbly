import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { CRMBackfillRequest } from "@/lib/api/models/app/crm/CRMProvider";
import getCrmBackfill from "@/lib/api/client/app/crm/provider/getCrmBackfill";
import startCrmBackfill from "@/lib/api/client/app/crm/provider/startCrmBackfill";

// How many Warmbly-only deals, tasks and notes a switch to HubSpot would copy.
export function useCrmBackfillPreview(enabled = true) {
    return useQuery({
        queryKey: ["crm", "backfill"],
        queryFn: getCrmBackfill,
        enabled,
    });
}

export function useStartCrmBackfill() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: CRMBackfillRequest) => startCrmBackfill(data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["crm", "backfill"] });
            queryClient.invalidateQueries({ queryKey: ["crm", "sync"] });
        },
    });
}
