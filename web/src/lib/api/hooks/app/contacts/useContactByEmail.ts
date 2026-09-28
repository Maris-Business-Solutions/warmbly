import { useQuery } from "@tanstack/react-query";
import lookupContact, { type ContactLookupThread } from "@/lib/api/client/app/contacts/lookupContact";

// Resolves a sender email to a contact (or null) for the unibox CRM panel; with
// the thread, a reply from an alias resolves to the campaign's lead.
export default function useContactByEmail(email: string | undefined, enabled = true, thread?: ContactLookupThread) {
    return useQuery({
        queryKey: ["contacts", "by-email", email ?? "", thread?.threadId ?? "", thread?.mailboxId ?? ""],
        queryFn: () => lookupContact(email as string, thread),
        enabled: enabled && !!email,
        staleTime: 60_000,
    });
}
