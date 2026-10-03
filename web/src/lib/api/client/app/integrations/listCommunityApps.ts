import type { CommunityAppsPage } from "@/lib/api/models/app/integrations/Community";
import Request from "../../Request";

export default async function listCommunityApps(): Promise<CommunityAppsPage> {
    return await Request<CommunityAppsPage>({
        method: "GET",
        url: "/integrations/community?limit=200",
        authorization: true,
    });
}
