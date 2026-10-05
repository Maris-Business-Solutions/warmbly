import { useMutation, useQueryClient } from "@tanstack/react-query";
import deleteContactNote from "@/lib/api/client/app/contacts/deleteContactNote";
import { patchQueries, removeEntities, restoreQueries } from "@/lib/api/hooks/optimistic";

// Optimistic: the note leaves on confirm and comes back if the server refuses.
export default function useDeleteContactNote() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: ({ contactId, noteId }: { contactId: string; noteId: string }) =>
            deleteContactNote(contactId, noteId),
        onMutate: ({ contactId, noteId }) =>
            patchQueries(queryClient, [["contacts", contactId, "notes"]], removeEntities([noteId])),
        onError: (_err, _vars, snapshot) => restoreQueries(queryClient, snapshot),
        onSettled: (_data, _err, variables) => {
            queryClient.invalidateQueries({
                queryKey: ["contacts", variables.contactId],
            })
        }
    })
}
