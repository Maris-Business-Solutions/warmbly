import type { CRMSettings } from "@/lib/api/models/app/crm/CRMProvider";
import Request from "../../../Request";

export default async function getCrmSettings(): Promise<CRMSettings> {
    return await Request<CRMSettings>({
        method: "GET",
        url: `/crm/settings`,
        authorization: true,
    })
}
