import { useMutation, useQueryClient } from "@tanstack/react-query";
import mapCrmOwner from "@/lib/api/client/app/crm/provider/mapCrmOwner";

export default function useMapCrmOwner() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ externalId, userId }: { externalId: string; userId: string | null }) => mapCrmOwner(externalId, userId),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["crm", "owners"] }),
    });
}
