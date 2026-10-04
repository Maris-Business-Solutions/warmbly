import Request from "../../../Request";

export default async function discardCrmSync(ids: string[] = []): Promise<{ affected: number }> {
    return await Request<{ affected: number }>({
        method: "POST",
        url: `/crm/sync/discard`,
        data: { ids },
        authorization: true,
    })
}
