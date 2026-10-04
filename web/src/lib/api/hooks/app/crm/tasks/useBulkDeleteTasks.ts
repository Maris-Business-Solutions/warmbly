import { useMutation, useQueryClient } from "@tanstack/react-query";
import bulkDeleteTasks from "@/lib/api/client/app/crm/tasks/bulkDeleteTasks";
import type TaskSelection from "@/lib/api/models/app/crm/TaskSelection";
import { useLivePatch } from "@/hooks/useLivePatch";
import { patchQueries, removeEntities, restoreQueries } from "@/lib/api/hooks/optimistic";
import { settleTasks, TASK_KEYS, taskMutationKey } from "./taskCache";

// Optimistic for ticked rows only; "all matching" reaches rows no page holds, so it waits for the refetch.
export default function useBulkDeleteTasks() {
    const queryClient = useQueryClient();
    const { pushPatch } = useLivePatch("crm_tasks");

    return useMutation({
        mutationKey: [...taskMutationKey, "bulk-delete"],
        mutationFn: (selection: TaskSelection) => bulkDeleteTasks(selection),
        onMutate: ({ all, tasks }) =>
            all || tasks.length === 0
                ? undefined
                : patchQueries(queryClient, TASK_KEYS, removeEntities(tasks)),
        onError: (_err, _selection, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: () => {
            pushPatch({ kind: "task_change" });
        },
        onSettled: () => settleTasks(queryClient),
    });
}
