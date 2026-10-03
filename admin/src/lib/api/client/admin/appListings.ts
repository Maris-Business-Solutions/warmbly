// /admin/app-listings: review queue for the community app directory.

import { Request } from "@/lib/api/client";
import { buildSearchQuery } from "@/lib/api/client/admin/query";
import type { AdminAppListing, AdminAppListingSearch, AdminAppListingsResult } from "@/lib/api/models/admin";

export function listAppListings(params: AdminAppListingSearch = {}): Promise<AdminAppListingsResult> {
    return Request({
        method: "GET",
        url: `/admin/app-listings${buildSearchQuery(params as Record<string, unknown>)}`,
        authorization: true,
    });
}

export function verifyAppListing(id: string, note: string): Promise<AdminAppListing> {
    return Request({
        method: "POST",
        url: `/admin/app-listings/${id}/verify`,
        authorization: true,
        data: { note },
    });
}

export function rejectAppListing(id: string, note: string): Promise<AdminAppListing> {
    return Request({
        method: "POST",
        url: `/admin/app-listings/${id}/reject`,
        authorization: true,
        data: { note },
    });
}
