import type { CRMBackfillRequest } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function startCrmBackfill(data: CRMBackfillRequest): Promise<void> {
    return await Request<void>({
        method: "POST",
        url: `/crm/backfill`,
        data: data,
        authorization: true,
    })
}
