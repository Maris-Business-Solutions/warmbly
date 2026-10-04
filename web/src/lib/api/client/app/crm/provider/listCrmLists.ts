import type { CRMListsResult } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export interface ListCrmListsParams {
    q?: string;
    cursor?: string;
    limit?: number;
}

export default async function listCrmLists(params: ListCrmListsParams = {}): Promise<CRMListsResult> {
    return await Request<CRMListsResult>({
        method: "GET",
        url: `/crm/lists`,
        params,
        authorization: true,
    })
}
