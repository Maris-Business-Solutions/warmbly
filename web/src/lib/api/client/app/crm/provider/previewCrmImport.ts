import type { CRMImportPreview, CRMImportRequest } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function previewCrmImport(data: CRMImportRequest): Promise<CRMImportPreview> {
    return await Request<CRMImportPreview>({
        method: "POST",
        url: `/crm/lists/preview`,
        data: data,
        authorization: true,
    })
}
