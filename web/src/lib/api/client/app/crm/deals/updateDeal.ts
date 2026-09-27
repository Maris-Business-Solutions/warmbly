import type Deal from "@/lib/api/models/app/crm/Deal";
import type { DealWrite } from "@/lib/api/models/app/crm/Deal";
import Request from "../../../Request";

export default async function updateDeal(id: string, data: DealWrite): Promise<Deal> {
    return await Request<Deal>({
        method: "PATCH",
        url: `/crm/deals/${id}`,
        data,
        authorization: true,
    })
}
