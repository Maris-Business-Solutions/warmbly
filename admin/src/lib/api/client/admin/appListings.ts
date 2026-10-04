// /admin/app-listings: the community app directory, featured, listed and hidden.

import { Request } from "@/lib/api/client";
import { buildSearchQuery } from "@/lib/api/client/admin/query";
import type {
    AdminAppListing,
    AdminAppListingSearch,
    AdminAppListingsResult,
    AppListingStatus,
} from "@/lib/api/models/admin";

export function listAppListings(params: AdminAppListingSearch = {}): Promise<AdminAppListingsResult> {
    return Request({
        method: "GET",
        url: `/admin/app-listings${buildSearchQuery(params as Record<string, unknown>)}`,
        authorization: true,
    });
}

export function setAppListingStatus(id: string, status: AppListingStatus, note: string): Promise<AdminAppListing> {
    return Request({
        method: "PUT",
        url: `/admin/app-listings/${id}/status`,
        authorization: true,
        data: { status, note },
    });
}
