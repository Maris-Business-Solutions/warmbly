import deleteCampaign from "@/lib/api/client/app/campaigns/deleteCampaign";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { patchQueries, removeEntities, restoreQueries, settle } from "@/lib/api/hooks/optimistic";
import { campaignMutationKey } from "./useStopCampaign";

// Deletes a campaign. The row leaves every cached list on the click and comes
// back if the server refuses. Its detail and analytics caches are dropped
// rather than invalidated (a refetch would only 404); the copy a mounted
// detail page still observes is left alone, since that page navigates away
// as soon as the delete resolves.
export default function useDeleteCampaign() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey: [...campaignMutationKey, "delete"],
        mutationFn: (id: string) => deleteCampaign(id),
        onMutate: (id) => patchQueries(queryClient, [["campaigns", "list"]], removeEntities([id])),
        onError: (_err, _id, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: (_data, id) => {
            queryClient.removeQueries({ queryKey: ["campaigns", id], type: "inactive" });
            queryClient.removeQueries({ queryKey: ["analytics", "campaigns", id], type: "inactive" });
            queryClient.invalidateQueries({ queryKey: ["analytics"] });
            queryClient.invalidateQueries({ queryKey: ["contacts"] });
        },
        onSettled: () => settle(queryClient, campaignMutationKey, [["campaigns", "list"]]),
    });
}
