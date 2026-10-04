import Request from "../../Request";

// Ends one member's authorization of an app in the current workspace.
export default async function revokeWorkspaceAuthorization(applicationId: string, userId: string): Promise<{ revoked: boolean }> {
    return await Request<{ revoked: boolean }>({
        method: "DELETE",
        url: `/oauth/workspace-authorizations/${applicationId}/members/${userId}`,
        authorization: true,
    });
}
