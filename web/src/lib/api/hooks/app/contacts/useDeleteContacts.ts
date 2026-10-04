import deleteContacts from "@/lib/api/client/app/contacts/deleteContacts";
import type ContactSelection from "@/lib/api/models/app/contacts/ContactSelection";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { patchQueries, removeEntities, restoreQueries } from "@/lib/api/hooks/optimistic";

// Optimistic for ticked rows only; "all matching" reaches rows no page holds, so it waits for the refetch.
export default function useDeleteContacts() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (selection: ContactSelection) => deleteContacts(selection),
        onMutate: ({ all, contacts }) =>
            all || contacts.length === 0
                ? undefined
                : patchQueries(queryClient, [["contacts", "list"]], removeEntities(contacts)),
        onError: (_err, _selection, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: (_, selection) => {
            selection.contacts.forEach(id => {
                queryClient.invalidateQueries({
                    queryKey: ["contacts", id]
                });
            });
            // The contacts table reads ["contacts","list",...]; refresh it so the
            // deleted rows disappear without a manual reload. ["campaigns"] carries
            // the lead counts the deleted contacts were part of.
            return Promise.all([
                queryClient.invalidateQueries({ queryKey: ["contacts", "list"] }),
                queryClient.invalidateQueries({ queryKey: ["campaigns"] }),
            ]);
        }
    })
}
