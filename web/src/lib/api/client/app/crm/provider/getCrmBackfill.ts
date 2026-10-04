import type { CRMBackfillPreview } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function getCrmBackfill(): Promise<CRMBackfillPreview> {
    return await Request<CRMBackfillPreview>({
        method: "GET",
        url: `/crm/backfill`,
        authorization: true,
    })
}
