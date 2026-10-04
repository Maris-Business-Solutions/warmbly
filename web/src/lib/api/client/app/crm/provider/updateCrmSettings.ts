import type { CRMSettings, UpdateCRMSettings } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function updateCrmSettings(data: UpdateCRMSettings): Promise<CRMSettings> {
    return await Request<CRMSettings>({
        method: "PUT",
        url: `/crm/settings`,
        data: data,
        authorization: true,
    })
}
