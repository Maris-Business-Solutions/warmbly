import { useMutation, useQueryClient } from "@tanstack/react-query";
import reorderTemplates from "@/lib/api/client/app/templates/reorderTemplates";
import type Template from "@/lib/api/models/app/templates/Template";
import { patchQueries, restoreQueries, settle } from "@/lib/api/hooks/optimistic";

const mutationKey = ["templates", "reorder"];

// Optimistic: the row moves on the click. Only the list that was reordered
// (the one holding exactly the sent ids) is patched; the rest re-read.
export default function useReorderTemplates() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey,
        mutationFn: (ids: string[]) => reorderTemplates(ids),
        onMutate: (ids) => {
            const rank = new Map(ids.map((id, i) => [id, i]));
            return patchQueries(queryClient, [["templates", "list"]], (data) => {
                const rows = data as Template[];
                if (!Array.isArray(rows) || rows.length !== ids.length || rows.some((t) => !rank.has(t?.id))) {
                    return data;
                }
                return [...rows]
                    .sort((a, b) => rank.get(a.id)! - rank.get(b.id)!)
                    .map((t, position) => ({ ...t, position }));
            });
        },
        onError: (_err, _ids, snapshot) => restoreQueries(queryClient, snapshot),
        onSettled: () => settle(queryClient, mutationKey, [["templates"]]),
    });
}
