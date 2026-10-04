import { useMutation, useQueryClient } from "@tanstack/react-query";
import type Deal from "@/lib/api/models/app/crm/Deal";
import type { DealWrite } from "@/lib/api/models/app/crm/Deal";
import updateDeal from "@/lib/api/client/app/crm/deals/updateDeal";
import { patchQueries, restoreQueries, settle, updateEntity } from "@/lib/api/hooks/optimistic";
import { applyDealWrite, cachedDeal, dealKeys, dealMutationKey, moveAcrossStages } from "./dealCache";

// Optimistic: a card dropped on another stage lands there on the drop, with
// the column counts moved; an edit shows in every list. Rolled back if refused.
export default function useUpdateDeal() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey: [...dealMutationKey, "update"],
        mutationFn: ({ id, data }: { id: string; data: DealWrite }) => updateDeal(id, data),
        onMutate: async ({ id, data }) => {
            const before = cachedDeal(queryClient, id);
            const keys = [...dealKeys(before?.contact_id), ["crm", "deals", "summary"]];
            const snapshot = await patchQueries(
                queryClient,
                keys,
                updateEntity<Deal>(id, (row) => applyDealWrite(queryClient, row, data)),
            );
            if (before && data.stage_id && data.stage_id !== before.stage_id) {
                moveAcrossStages(queryClient, before, applyDealWrite(queryClient, before, data));
            }
            return snapshot;
        },
        onError: (_err, _vars, snapshot) => restoreQueries(queryClient, snapshot),
        // Deals also render inside contacts (panel list + 360 timeline), so a
        // stage move / edit must refresh any active contact queries too.
        onSettled: () => settle(queryClient, dealMutationKey, [["crm", "deals"], ["contacts"]]),
    })
}
