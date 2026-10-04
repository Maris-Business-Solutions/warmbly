import { useMutation, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import type Campaign from "@/lib/api/models/app/campaigns/Campaign";
import updateCampaign from "@/lib/api/client/app/campaigns/updateCampaign";
import type GetCampaigns from "@/lib/api/models/app/campaigns/GetCampaigns";
import { patchQueries, removeEntities, restoreQueries, settle, updateEntity } from "@/lib/api/hooks/optimistic";
import { campaignMutationKey } from "./useStopCampaign";

// Only fields whose saved value is exactly what was sent; schedule and limits wait for the server.
function predictable(campaign: Partial<Campaign>): Partial<Campaign> {
    const out: Partial<Campaign> = {};
    if (campaign.name !== undefined) out.name = campaign.name;
    if (campaign.description !== undefined) out.description = campaign.description;
    if (campaign.folders !== undefined) out.folders = campaign.folders;
    return out;
}

export default function useUpdateCampaign(id: string) {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey: [...campaignMutationKey, "update", id],
        mutationFn: (campaign: Partial<Campaign>) =>
            updateCampaign(id, campaign),
        // Optimistic for a rename and folder moves; a list scoped to a folder the campaign left drops the row.
        onMutate: (campaign) => {
            const patch = predictable(campaign);
            if (Object.keys(patch).length === 0) return undefined;
            const update = updateEntity<Campaign>(id, (row) => ({ ...row, ...patch }));
            const leave = removeEntities([id]);
            return patchQueries(queryClient, [["campaigns", "list"], ["campaigns", id]], (data, key) => {
                const folder = key[1] === "list" ? key[3] : undefined;
                if (patch.folders && typeof folder === "string" && folder && !patch.folders.includes(folder)) {
                    return leave(data, key);
                }
                return update(data, key);
            });
        },
        onError: (_err, _campaign, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: (data) => {
            const allLists = queryClient.getQueriesData<InfiniteData<GetCampaigns>>({
                queryKey: ["campaigns", "list"],
            });

            for (const [key, oldData] of allLists) {
                if (!oldData) continue;

                queryClient.setQueryData(key, {
                    ...oldData,
                    pages: oldData.pages.map((page) => ({
                        ...page,
                        data: page.data.map((c) => c.id === id ? data : c),
                    })),
                });
            }

            queryClient.setQueryData<Campaign>(
                ["campaigns", id],
                data
            );
        },
        // Folder membership decides which lists hold the campaign at all, so only a refetch can add it.
        // Always settles, so a re-read another campaign write deferred behind this one still runs.
        onSettled: (_data, _err, campaign) => {
            settle(queryClient, campaignMutationKey, campaign.folders !== undefined ? [["campaigns", "list"]] : []);
        },
    })
}
