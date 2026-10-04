// Community app directory (mirror of internal/models/app_directory.go).

// published: link only, listed once enough workspaces use it. featured: picked
// by the team and always listed. hidden: taken down, reachable nowhere.
export type AppListingStatus = "published" | "featured" | "hidden";

export type AppListingCategory =
    | "crm"
    | "automation"
    | "notifications"
    | "meetings"
    | "data"
    | "verification"
    | "ai"
    | "other";

// A published app as a workspace browsing the directory sees it.
export interface CommunityApp {
    application_id: string;
    slug: string;
    name: string;
    tagline: string;
    description: string;
    category: AppListingCategory;
    logo_url: string;
    website_url: string;
    install_url: string;
    support_url: string;
    privacy_url: string;
    /** The publishing workspace's name. */
    developer: string;
    /** API-permission bitmask the app may request (same bits as API keys). */
    scopes: number;
    /** The same permissions, spelled out. */
    permissions: { name: string; value: number; description: string; category: string }[];
    status: Exclude<AppListingStatus, "hidden">;
    /** Shown in the directory: featured, or used by enough workspaces. */
    listed: boolean;
    /** Workspaces on this instance with an active grant. */
    installs: number;
    /** Whether anyone in the caller's workspace has authorized it. */
    installed: boolean;
    published_at: Date;
}

export interface CommunityAppsPage {
    data: CommunityApp[];
    pagination: { total: number | null; next_cursor?: string | null; has_more: boolean };
}

// The developer's own listing of one of their OAuth apps.
export interface AppListing {
    application_id: string;
    organization_id: string;
    slug: string;
    tagline: string;
    description: string;
    category: AppListingCategory;
    install_url: string;
    support_url: string;
    privacy_url: string;
    status: AppListingStatus;
    /** Why the team hid it, shown to the developer. */
    status_note?: string;
    status_at?: Date;
    submitted_at: Date;
    created_at: Date;
    updated_at: Date;
}

export interface AppListingInput {
    slug: string;
    tagline: string;
    description: string;
    category: AppListingCategory;
    install_url: string;
    support_url: string;
    privacy_url: string;
}

export const LISTING_CATEGORY_LABELS: Record<AppListingCategory, string> = {
    crm: "CRM",
    automation: "Automation",
    notifications: "Notifications",
    meetings: "Meetings",
    data: "Data",
    verification: "Verification",
    ai: "AI",
    other: "Other",
};

// Workspaces that must use a published app before it is listed (config.AppDirectoryPopularInstalls).
export const POPULAR_INSTALLS = 25;

// The share link a developer hands out; it opens the listing drawer.
export function communityAppPath(slug: string): string {
    return `/app/integrations/apps/${slug}`;
}
