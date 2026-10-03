import type { AppListing, AppListingInput } from "@/lib/api/models/app/integrations/Community";
import Request from "../../Request";

export async function getAppListing(appId: string): Promise<{ listing: AppListing | null }> {
    return await Request<{ listing: AppListing | null }>({
        method: "GET",
        url: `/oauth/applications/${appId}/listing`,
        authorization: true,
    });
}

export async function saveAppListing(appId: string, input: AppListingInput): Promise<{ listing: AppListing }> {
    return await Request<{ listing: AppListing }>({
        method: "PUT",
        url: `/oauth/applications/${appId}/listing`,
        authorization: true,
        data: input,
    });
}

export async function deleteAppListing(appId: string): Promise<{ deleted: boolean }> {
    return await Request<{ deleted: boolean }>({
        method: "DELETE",
        url: `/oauth/applications/${appId}/listing`,
        authorization: true,
    });
}
