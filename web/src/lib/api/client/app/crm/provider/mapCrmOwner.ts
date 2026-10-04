import Request from "../../../Request";

export default async function mapCrmOwner(externalId: string, userId: string | null): Promise<void> {
    return await Request<void>({
        method: "PUT",
        url: `/crm/owners/${encodeURIComponent(externalId)}`,
        data: { user_id: userId },
        authorization: true,
    })
}
