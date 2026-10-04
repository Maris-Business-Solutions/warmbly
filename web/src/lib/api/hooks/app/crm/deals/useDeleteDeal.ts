import { useMutation, useQueryClient } from "@tanstack/react-query";
import deleteDeal from "@/lib/api/client/app/crm/deals/deleteDeal";
import { patchQueries, removeEntities, restoreQueries, settle } from "@/lib/api/hooks/optimistic";
import { cachedDeal, dealKeys, dealMutationKey } from "./dealCache";

// Optimistic: the card leaves every list and board on the click and comes back if refused.
export default function useDeleteDeal() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey: [...dealMutationKey, "delete"],
        mutationFn: (id: string) => deleteDeal(id),
        onMutate: (id) =>
            patchQueries(queryClient, dealKeys(cachedDeal(queryClient, id)?.contact_id), removeEntities([id])),
        onError: (_err, _id, snapshot) => restoreQueries(queryClient, snapshot),
        // Deals also render inside contacts (panel list + 360 timeline).
        onSettled: () => settle(queryClient, dealMutationKey, [["crm", "deals"], ["contacts"]]),
    })
}
