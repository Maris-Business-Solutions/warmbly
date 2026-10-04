import type { CRMOwner } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function listCrmOwners(): Promise<{ data: CRMOwner[] }> {
    return await Request<{ data: CRMOwner[] }>({
        method: "GET",
        url: `/crm/owners`,
        authorization: true,
    })
}
