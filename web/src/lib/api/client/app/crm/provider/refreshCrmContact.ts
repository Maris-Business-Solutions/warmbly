import type { CRMContactView } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function refreshCrmContact(contactId: string): Promise<CRMContactView> {
    return await Request<CRMContactView>({
        method: "POST",
        url: `/crm/contacts/${contactId}/refresh`,
        authorization: true,
    })
}
