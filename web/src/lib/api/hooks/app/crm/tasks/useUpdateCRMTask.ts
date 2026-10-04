import { useMutation, useQueryClient } from "@tanstack/react-query";
import type CRMTask from "@/lib/api/models/app/crm/CRMTask";
import type { CRMTaskWrite } from "@/lib/api/models/app/crm/CRMTask";
import updateCRMTask from "@/lib/api/client/app/crm/tasks/updateCRMTask";
import { useLivePatch } from "@/hooks/useLivePatch";
import { patchQueries, restoreQueries, updateEntity } from "@/lib/api/hooks/optimistic";
import { applyTaskWrite, settleTasks, TASK_KEYS, taskMutationKey } from "./taskCache";

export default function useUpdateCRMTask() {
    const queryClient = useQueryClient();
    // Nudge teammates on the tasks view to refresh instantly (e.g. a status
    // toggle), ahead of the durable audit refetch. Send-only (no subscription).
    const { pushPatch } = useLivePatch("crm_tasks");

    return useMutation({
        mutationKey: [...taskMutationKey, "update"],
        mutationFn: ({ id, data }: { id: string; data: CRMTaskWrite }) => updateCRMTask(id, data),
        // Optimistic: the checkbox, priority or title changes on the click; the summary waits for the refetch.
        onMutate: ({ id, data }) =>
            patchQueries(queryClient, TASK_KEYS, updateEntity<CRMTask>(id, (row) => applyTaskWrite(row, data))),
        onError: (_err, _vars, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: () => {
            pushPatch({ kind: "task_change" })
        },
        // Broad prefix: the search lists, the summary totals and the sidebar overdue indicator.
        onSettled: () => settleTasks(queryClient),
    })
}
