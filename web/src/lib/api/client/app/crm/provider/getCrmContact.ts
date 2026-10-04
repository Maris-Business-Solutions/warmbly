import type { CRMContactView } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function getCrmContact(contactId: string): Promise<CRMContactView> {
    return await Request<CRMContactView>({
        method: "GET",
        url: `/crm/contacts/${contactId}`,
        authorization: true,
    })
}
