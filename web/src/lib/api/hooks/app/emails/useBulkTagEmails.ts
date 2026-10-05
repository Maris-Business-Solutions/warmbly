import bulkTagEmails from "@/lib/api/client/app/emails/bulkTagEmails";
import type Inbox from "@/lib/api/models/app/emails/Inbox";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { patchQueries, restoreQueries, settle, updateEntities } from "@/lib/api/hooks/optimistic";

interface BulkTagInput {
    emailIds: string[];
    addTags: string[];
    removeTags: string[];
}

const mutationKey = ["emails", "bulk-tag"];

// Optimistic: the chips change on every selected row at once; the refetch confirms.
export default function useBulkTagEmails() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationKey,
        mutationFn: ({ emailIds, addTags, removeTags }: BulkTagInput) =>
            bulkTagEmails(emailIds, addTags, removeTags),
        onMutate: ({ emailIds, addTags, removeTags }) => {
            const drop = new Set(removeTags);
            return patchQueries(
                queryClient,
                [["emails", "list"]],
                updateEntities<Inbox>(emailIds, (row) => {
                    const kept = (row.tags ?? []).filter((t) => !drop.has(t));
                    return { ...row, tags: [...kept, ...addTags.filter((t) => !kept.includes(t))] };
                }),
            );
        },
        onError: (_err, _input, snapshot) => restoreQueries(queryClient, snapshot),
        // Tag membership changed on many rows; refetch lists and details.
        onSettled: () => settle(queryClient, mutationKey, [["emails"]]),
    })
}
