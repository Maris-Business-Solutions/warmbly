import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UpdateCRMContact } from "@/lib/api/models/app/crm/CRMProvider";
import getCrmContact from "@/lib/api/client/app/crm/provider/getCrmContact";
import refreshCrmContact from "@/lib/api/client/app/crm/provider/refreshCrmContact";
import linkCrmContact from "@/lib/api/client/app/crm/provider/linkCrmContact";
import updateCrmContact from "@/lib/api/client/app/crm/provider/updateCrmContact";

// The HubSpot side of a contact. Opening it asks the server to pull the
// contact's record, deals, tasks and notes (debounced server-side), so the
// panel is current without anyone pressing refresh.
export function useCrmContact(contactId: string | undefined, enabled = true) {
    const queryClient = useQueryClient();
    const query = useQuery({
        queryKey: ["crm", "contact", contactId],
        queryFn: () => getCrmContact(contactId as string),
        enabled: enabled && !!contactId,
        staleTime: 30_000,
    });
    useEffect(() => {
        if (!enabled || !contactId) return;
        let cancelled = false;
        refreshCrmContact(contactId)
            .then((view) => {
                if (cancelled) return;
                queryClient.setQueryData(["crm", "contact", contactId], view);
                queryClient.invalidateQueries({ queryKey: ["contacts", contactId] });
            })
            .catch(() => {});
        return () => {
            cancelled = true;
        };
    }, [contactId, enabled, queryClient]);
    return query;
}

export function useLinkCrmContact() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (contactId: string) => linkCrmContact(contactId),
        onSuccess: (view, contactId) => {
            queryClient.setQueryData(["crm", "contact", contactId], view);
            queryClient.invalidateQueries({ queryKey: ["contacts", contactId] });
        },
    });
}

export function useUpdateCrmContact() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ contactId, data }: { contactId: string; data: UpdateCRMContact }) => updateCrmContact(contactId, data),
        onSuccess: (view, { contactId }) => {
            queryClient.setQueryData(["crm", "contact", contactId], view);
        },
    });
}
