import { useQuery } from "@tanstack/react-query";
import { getContactImport, listContactImports } from "@/lib/api/client/app/contacts/contactImports";
import { isImportActive } from "@/lib/api/models/app/contacts/ContactImport";

export const CONTACT_IMPORTS_KEY = ["contacts", "imports"] as const;

// Recent imports; CONTACT_IMPORT_PROGRESS keeps them live, and a running one
// is also re-read every few seconds in case the socket is down.
export function useContactImports(enabled = true, limit = 10) {
    return useQuery({
        queryKey: [...CONTACT_IMPORTS_KEY, "list", limit],
        queryFn: () => listContactImports(limit),
        enabled,
        staleTime: 15_000,
        refetchInterval: (q) => (q.state.data?.data?.some((i) => isImportActive(i.status)) ? 5_000 : false),
    });
}

export function useContactImport(id: string | null) {
    return useQuery({
        queryKey: [...CONTACT_IMPORTS_KEY, id],
        queryFn: () => getContactImport(id as string),
        enabled: !!id,
        refetchInterval: (q) => (q.state.data && isImportActive(q.state.data.status) ? 4_000 : false),
    });
}
