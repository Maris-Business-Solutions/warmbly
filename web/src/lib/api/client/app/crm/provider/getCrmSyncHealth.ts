import type { CRMSyncHealth } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function getCrmSyncHealth(): Promise<CRMSyncHealth> {
    return await Request<CRMSyncHealth>({
        method: "GET",
        url: `/crm/sync`,
        authorization: true,
    })
}
