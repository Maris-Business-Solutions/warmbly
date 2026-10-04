import Request from "../../../Request";

export default async function retryCrmSync(ids: string[] = []): Promise<{ affected: number }> {
    return await Request<{ affected: number }>({
        method: "POST",
        url: `/crm/sync/retry`,
        data: { ids },
        authorization: true,
    })
}
