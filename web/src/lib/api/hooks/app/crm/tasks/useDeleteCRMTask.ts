import { useMutation, useQueryClient } from "@tanstack/react-query";
import deleteCRMTask from "@/lib/api/client/app/crm/tasks/deleteCRMTask";
import { useLivePatch } from "@/hooks/useLivePatch";
import { patchQueries, removeEntities, restoreQueries } from "@/lib/api/hooks/optimistic";
import { settleTasks, TASK_KEYS, taskMutationKey } from "./taskCache";

// Optimistic: the row leaves every task list on the click and comes back if the server refuses.
export default function useDeleteCRMTask() {
    const queryClient = useQueryClient();
    const { pushPatch } = useLivePatch("crm_tasks");

    return useMutation({
        mutationKey: [...taskMutationKey, "delete"],
        mutationFn: (id: string) => deleteCRMTask(id),
        onMutate: (id) => patchQueries(queryClient, TASK_KEYS, removeEntities([id])),
        onError: (_err, _id, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: () => {
            pushPatch({ kind: "task_change" })
        },
        onSettled: () => settleTasks(queryClient),
    })
}
