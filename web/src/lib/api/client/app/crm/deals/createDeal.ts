import type Deal from "@/lib/api/models/app/crm/Deal";
import type { DealWrite } from "@/lib/api/models/app/crm/Deal";
import Request from "../../../Request";

export default async function createDeal(data: DealWrite): Promise<Deal> {
    return await Request<Deal>({
        method: "POST",
        url: `/crm/deals`,
        data,
        authorization: true,
    })
}
