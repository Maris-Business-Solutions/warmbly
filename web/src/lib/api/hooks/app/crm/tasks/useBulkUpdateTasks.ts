import { useMutation, useQueryClient } from "@tanstack/react-query";
import bulkUpdateTasks from "@/lib/api/client/app/crm/tasks/bulkUpdateTasks";
import type CRMTask from "@/lib/api/models/app/crm/CRMTask";
import type { BulkUpdateTasks } from "@/lib/api/models/app/crm/TaskSelection";
import { useLivePatch } from "@/hooks/useLivePatch";
import { patchQueries, restoreQueries, updateEntities } from "@/lib/api/hooks/optimistic";
import { applyTaskWrite, settleTasks, TASK_KEYS, taskMutationKey } from "./taskCache";

// Optimistic for ticked rows only; "all matching" reaches rows no page holds, so it waits for the refetch.
export default function useBulkUpdateTasks() {
    const queryClient = useQueryClient();
    const { pushPatch } = useLivePatch("crm_tasks");

    return useMutation({
        mutationKey: [...taskMutationKey, "bulk-update"],
        mutationFn: (data: BulkUpdateTasks) => bulkUpdateTasks(data),
        onMutate: ({ all, tasks, status, priority }) => {
            if (all || tasks.length === 0) return undefined;
            const write = { ...(status ? { status } : {}), ...(priority ? { priority } : {}) };
            return patchQueries(
                queryClient,
                TASK_KEYS,
                updateEntities<CRMTask>(tasks, (row) => applyTaskWrite(row, write)),
            );
        },
        onError: (_err, _data, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: () => {
            pushPatch({ kind: "task_change" });
        },
        onSettled: () => settleTasks(queryClient),
    });
}
