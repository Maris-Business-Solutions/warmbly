import type Contact from "@/lib/api/models/app/contacts/Contact";
import Request from "../../Request";

// "email": the sender's address is the contact's. "thread": the thread answers
// a campaign send to the contact and the reply came from another address.
export type ContactLookupMatch = "email" | "thread";

export interface ContactLookup {
    contact: Contact | null;
    match?: ContactLookupMatch;
}

// The conversation the sender wrote in, so a reply from an alias still
// resolves to the lead the campaign wrote to.
export interface ContactLookupThread {
    threadId?: string;
    mailboxId?: string;
}

// Resolve a sender address to a contact in the current organization. A null
// contact means the sender isn't a known contact (a normal, non-error outcome).
export default async function lookupContact(email: string, thread?: ContactLookupThread): Promise<ContactLookup> {
    const params = new URLSearchParams({ email });
    if (thread?.threadId) params.set("thread_id", thread.threadId);
    if (thread?.mailboxId) params.set("account_id", thread.mailboxId);
    const res = await Request<ContactLookup>({
        method: "GET",
        url: `/contacts/lookup?${params.toString()}`,
        authorization: true,
    });
    return { contact: res?.contact ?? null, match: res?.match };
}
