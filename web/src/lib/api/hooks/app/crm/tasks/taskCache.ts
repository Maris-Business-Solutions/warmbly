import type { QueryClient } from "@tanstack/react-query";
import type CRMTask from "@/lib/api/models/app/crm/CRMTask";
import type { CRMTaskWrite } from "@/lib/api/models/app/crm/CRMTask";
import { settle } from "@/lib/api/hooks/optimistic";

// Every task write shares this prefix, so the lists re-read once the last one settles.
export const taskMutationKey = ["crm", "tasks"];
export const TASK_KEYS = [["crm", "tasks"]];

// A write as the row will read once the server has it: the due date revived,
// and completed_at set or cleared with the status.
export function applyTaskWrite(row: CRMTask, data: CRMTaskWrite): CRMTask {
    const { due_date, ...rest } = data;
    const next: CRMTask = { ...row, ...rest, id: row.id };
    if ("due_date" in data) next.due_date = due_date ? new Date(due_date) : undefined;
    if (data.status && data.status !== row.status) {
        next.completed_at = data.status === "completed" ? new Date() : undefined;
    }
    return next;
}

export function settleTasks(queryClient: QueryClient) {
    settle(queryClient, taskMutationKey, TASK_KEYS);
}
