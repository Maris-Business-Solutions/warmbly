import { useMutation, useQueryClient } from "@tanstack/react-query";
import deleteTemplate from "@/lib/api/client/app/templates/deleteTemplate";
import { patchQueries, removeEntities, restoreQueries } from "@/lib/api/hooks/optimistic";

// Optimistic: the row leaves on confirm and comes back if the server refuses.
export default function useDeleteTemplate() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (id: string) => deleteTemplate(id),
        onMutate: (id) => patchQueries(queryClient, [["templates", "list"]], removeEntities([id])),
        onError: (_err, _id, snapshot) => restoreQueries(queryClient, snapshot),
        onSettled: () => {
            queryClient.invalidateQueries({
                queryKey: ["templates"],
            })
        }
    })
}
