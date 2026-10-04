import type { CRMContactView } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function linkCrmContact(contactId: string): Promise<CRMContactView> {
    return await Request<CRMContactView>({
        method: "POST",
        url: `/crm/contacts/${contactId}/link`,
        authorization: true,
    })
}
