import { useMutation, useQueryClient } from "@tanstack/react-query";
import type ContactNote from "@/lib/api/models/app/crm/ContactNote";
import updateContactNote from "@/lib/api/client/app/contacts/updateContactNote";
import { patchQueries, restoreQueries, updateEntity } from "@/lib/api/hooks/optimistic";

// Optimistic: the edited text shows on save; the refetch brings the server's copy.
export default function useUpdateContactNote() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: ({ contactId, noteId, data }: { contactId: string; noteId: string; data: Partial<ContactNote> }) =>
            updateContactNote(contactId, noteId, data),
        onMutate: ({ contactId, noteId, data }) =>
            patchQueries(
                queryClient,
                [["contacts", contactId, "notes"]],
                updateEntity<ContactNote>(noteId, (note) => ({ ...note, ...data, id: noteId, updated_at: new Date() })),
            ),
        onError: (_err, _vars, snapshot) => restoreQueries(queryClient, snapshot),
        onSettled: (_data, _err, variables) => {
            queryClient.invalidateQueries({
                queryKey: ["contacts", variables.contactId],
            })
        }
    })
}
