import { useMutation, useQueryClient } from "@tanstack/react-query";
import stopCampaign from "@/lib/api/client/app/campaigns/stopCampaign";
import type Campaign from "@/lib/api/models/app/campaigns/Campaign";
import { patchQueries, restoreQueries, settle, updateEntity } from "@/lib/api/hooks/optimistic";

export const campaignMutationKey = ["campaigns"];

// Optimistic: a stop always lands on paused, so the row and the detail show it on the click.
export default function useStopCampaign() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey: [...campaignMutationKey, "stop"],
        mutationFn: (id: string) => stopCampaign(id),
        onMutate: (id) =>
            patchQueries(
                queryClient,
                [["campaigns", "list"], ["campaigns", id]],
                updateEntity<Campaign>(id, (row) => ({ ...row, status: "paused", idle_since: null })),
            ),
        onError: (_err, _id, snapshot) => restoreQueries(queryClient, snapshot),
        onSettled: () => settle(queryClient, campaignMutationKey, [["campaigns"]]),
    })
}
