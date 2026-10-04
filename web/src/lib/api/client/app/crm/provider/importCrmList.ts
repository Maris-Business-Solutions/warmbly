import type { CRMImportRequest, CRMImportResult } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function importCrmList(data: CRMImportRequest): Promise<CRMImportResult> {
    return await Request<CRMImportResult>({
        method: "POST",
        url: `/crm/lists/import`,
        data: data,
        authorization: true,
    })
}
