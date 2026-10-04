import type { CRMContactView, UpdateCRMContact } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function updateCrmContact(contactId: string, data: UpdateCRMContact): Promise<CRMContactView> {
    return await Request<CRMContactView>({
        method: "PATCH",
        url: `/crm/contacts/${contactId}`,
        data: data,
        authorization: true,
    })
}
