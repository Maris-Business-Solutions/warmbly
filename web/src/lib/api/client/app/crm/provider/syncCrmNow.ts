import Request from "../../../Request";

export default async function syncCrmNow(): Promise<void> {
    return await Request<void>({
        method: "POST",
        url: `/crm/sync`,
        authorization: true,
    })
}
