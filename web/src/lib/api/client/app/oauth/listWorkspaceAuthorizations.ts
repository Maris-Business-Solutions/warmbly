import Request from "../../Request";
import type { OAuthWorkspaceAuthorizationsResult } from "@/lib/api/models/app/oauth/OAuthApp";

// Every member's live app authorizations in the current workspace.
export default async function listWorkspaceAuthorizations(): Promise<OAuthWorkspaceAuthorizationsResult> {
    return await Request<OAuthWorkspaceAuthorizationsResult>({
        method: "GET",
        url: `/oauth/workspace-authorizations`,
        authorization: true,
    });
}
