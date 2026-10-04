// /admin/oauth-apps and /admin/oauth-developer-blocks: moderating OAuth apps
// and who may build them.

import { Request } from "@/lib/api/client";
import { buildSearchQuery } from "@/lib/api/client/admin/query";
import type { AdminOAuthApp, AdminOAuthAppSearch, AdminOAuthAppsResult, OAuthDeveloperBlock } from "@/lib/api/models/admin";

export function listOAuthApps(params: AdminOAuthAppSearch = {}): Promise<AdminOAuthAppsResult> {
    return Request({ method: "GET", url: `/admin/oauth-apps${buildSearchQuery(params as Record<string, unknown>)}`, authorization: true });
}

export function suspendOAuthApp(id: string, reason: string): Promise<AdminOAuthApp> {
    return Request({ method: "POST", url: `/admin/oauth-apps/${id}/suspend`, authorization: true, data: { reason } });
}

export function unsuspendOAuthApp(id: string): Promise<AdminOAuthApp> {
    return Request({ method: "POST", url: `/admin/oauth-apps/${id}/unsuspend`, authorization: true });
}

export function revokeOAuthAppGrants(id: string): Promise<{ revoked: number }> {
    return Request({ method: "POST", url: `/admin/oauth-apps/${id}/revoke-grants`, authorization: true });
}

export function removeOAuthAppLogo(id: string): Promise<AdminOAuthApp> {
    return Request({ method: "POST", url: `/admin/oauth-apps/${id}/remove-logo`, authorization: true });
}

export function listDeveloperBlocks(): Promise<{ data: OAuthDeveloperBlock[] }> {
    return Request({ method: "GET", url: `/admin/oauth-developer-blocks`, authorization: true });
}

export function createDeveloperBlock(body: {
    organization_id?: string;
    user_id?: string;
    reason: string;
    suspend_apps: boolean;
}): Promise<{ block: OAuthDeveloperBlock; suspended_apps: number }> {
    return Request({ method: "POST", url: `/admin/oauth-developer-blocks`, authorization: true, data: body });
}

export function deleteDeveloperBlock(id: string): Promise<{ deleted: boolean }> {
    return Request({ method: "DELETE", url: `/admin/oauth-developer-blocks/${id}`, authorization: true });
}
