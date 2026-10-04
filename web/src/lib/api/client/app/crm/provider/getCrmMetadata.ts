import type { CRMMetadata } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function getCrmMetadata(): Promise<CRMMetadata> {
    return await Request<CRMMetadata>({
        method: "GET",
        url: `/crm/metadata`,
        authorization: true,
    })
}
