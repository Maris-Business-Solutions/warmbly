import { useQuery } from "@tanstack/react-query";
import estimateCampaign, { type CampaignEstimateInput } from "@/lib/api/client/app/campaigns/estimateCampaign";

// Audience-versus-pool projection for the campaign wizard. Keyed on the whole
// input so every edit (segment, tag, limit, window, waits, start) re-asks. With
// no segment it still answers for the pool: its mailboxes and capacity.
export default function useCampaignEstimate(input: CampaignEstimateInput, enabled = true) {
    return useQuery({
        queryKey: ["campaigns", "estimate", input],
        queryFn: () => estimateCampaign(input),
        enabled,
        staleTime: 30_000,
        placeholderData: (prev) => prev,
    });
}
