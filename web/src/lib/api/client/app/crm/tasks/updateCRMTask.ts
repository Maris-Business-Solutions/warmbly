import type CRMTask from "@/lib/api/models/app/crm/CRMTask";
import type { CRMTaskWrite } from "@/lib/api/models/app/crm/CRMTask";
import Request from "../../../Request";

export default async function updateCRMTask(id: string, data: CRMTaskWrite): Promise<CRMTask> {
    return await Request<CRMTask>({
        method: "PATCH",
        url: `/crm/tasks/${id}`,
        data,
        authorization: true,
    })
}
