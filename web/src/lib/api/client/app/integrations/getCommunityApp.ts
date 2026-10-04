import type { CommunityApp } from "@/lib/api/models/app/integrations/Community";
import Request from "../../Request";

export default async function getCommunityApp(slug: string): Promise<CommunityApp> {
    return await Request<CommunityApp>({
        method: "GET",
        url: `/integrations/community/${encodeURIComponent(slug)}`,
        authorization: true,
    });
}
