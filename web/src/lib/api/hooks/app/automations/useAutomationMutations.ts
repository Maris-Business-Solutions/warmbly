import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import createAutomation from "@/lib/api/client/app/automations/createAutomation";
import updateAutomation from "@/lib/api/client/app/automations/updateAutomation";
import deleteAutomation from "@/lib/api/client/app/automations/deleteAutomation";
import testAutomation from "@/lib/api/client/app/automations/testAutomation";
import type { Automation, AutomationWrite } from "@/lib/api/models/app/automations/Automation";
import { restoreQueries, settle, type Snapshot } from "@/lib/api/hooks/optimistic";

const LIST_KEY = ["automations"];
const MUTATION_KEY = ["automations"];

export function useTestAutomation() {
    return useMutation({
        mutationFn: ({ id, data, skipNodeIds }: { id: string; data?: Record<string, unknown>; skipNodeIds?: string[] }) =>
            testAutomation(id, data, skipNodeIds),
    });
}

export function useCreateAutomation() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (w: AutomationWrite) => createAutomation(w),
        onSuccess: () => void qc.invalidateQueries({ queryKey: ["automations"] }),
    });
}

type AutomationList = { automations: Automation[] };

// Patches the list query alone; the open editor's own copy is left to its save.
async function patchList(qc: QueryClient, fn: (rows: Automation[]) => Automation[]): Promise<Snapshot> {
    await qc.cancelQueries({ queryKey: LIST_KEY, exact: true });
    const previous = qc.getQueryData<AutomationList>(LIST_KEY);
    if (!Array.isArray(previous?.automations)) return [];
    qc.setQueryData<AutomationList>(LIST_KEY, { ...previous, automations: fn(previous.automations) });
    return [[LIST_KEY, previous, qc.getQueryData(LIST_KEY)]];
}

// Optimistic on the list: the on/off pill and the name change on the click.
export function useUpdateAutomation() {
    const qc = useQueryClient();
    return useMutation({
        mutationKey: [...MUTATION_KEY, "update"],
        mutationFn: ({ id, w }: { id: string; w: AutomationWrite }) => updateAutomation(id, w),
        onMutate: ({ id, w }) =>
            patchList(qc, (rows) => rows.map((a) => (a.id === id ? { ...a, name: w.name, enabled: w.enabled } : a))),
        onError: (_err, _vars, snapshot) => restoreQueries(qc, snapshot),
        // The prefix covers the list and the automation's own query.
        onSettled: () => settle(qc, MUTATION_KEY, [LIST_KEY]),
    });
}

// Optimistic: the card leaves on confirm and comes back if the server refuses (a step still uses it).
export function useDeleteAutomation() {
    const qc = useQueryClient();
    return useMutation({
        mutationKey: [...MUTATION_KEY, "delete"],
        mutationFn: (id: string) => deleteAutomation(id),
        onMutate: (id) => patchList(qc, (rows) => rows.filter((a) => a.id !== id)),
        onError: (_err, _id, snapshot) => restoreQueries(qc, snapshot),
        onSettled: () => settle(qc, MUTATION_KEY, [LIST_KEY]),
    });
}
