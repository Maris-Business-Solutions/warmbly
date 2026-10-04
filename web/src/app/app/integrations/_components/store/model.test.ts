import { describe, expect, it } from "vitest";

import type { CommunityApp } from "@/lib/api/models/app/integrations/Community";
import type { IntegrationCatalogEntry } from "@/lib/api/models/app/integrations/Integration";

import {
    applyFilters,
    builtinItem,
    communityItem,
    DEFAULT_FILTERS,
    highlightParts,
    readFilters,
    searchItems,
    withQuery,
    writeFilters,
    type StoreItem,
} from "./model";

function entry(provider: string, name: string, category: string, rank: number, extra: Partial<IntegrationCatalogEntry> = {}) {
    return builtinItem({
        provider,
        name,
        tagline: `${name} tagline`,
        category,
        auth_method: "oauth",
        beta: false,
        supports_push: false,
        configured: true,
        rank,
        ...extra,
    } as IntegrationCatalogEntry);
}

function app(slug: string, name: string, category: string, installs: number, status: "published" | "featured" = "published", published = "2026-01-01") {
    return communityItem({
        application_id: slug,
        slug,
        name,
        tagline: `${name} does things`,
        description: "",
        category,
        logo_url: "",
        website_url: "",
        install_url: "https://x.example",
        support_url: "",
        privacy_url: "",
        developer: "Acme",
        scopes: 0,
        permissions: [],
        status,
        listed: true,
        installs,
        installed: false,
        published_at: new Date(published),
    } as unknown as CommunityApp);
}

const items: StoreItem[] = [
    entry("hubspot", "HubSpot", "crm", 1),
    entry("slack", "Slack", "notifications", 2),
    entry("calendly", "Calendly", "meetings", 3),
    entry("salesforce", "Salesforce", "crm", 4, { configured: false }),
    entry("close", "Close", "crm", 9, { auth_method: "api_key" }),
    app("pipeline-pulse", "Pipeline Pulse", "crm", 30, "featured", "2026-03-01"),
    app("reply-radar", "Reply Radar", "notifications", 50, "published", "2026-05-01"),
];
const names = (list: StoreItem[]) => list.map((i) => i.name);
const none = () => false;

describe("store search", () => {
    it("ranks a name match above a tagline match", () => {
        expect(names(searchItems(items, "slack"))[0]).toBe("Slack");
    });
    it("requires every word to match", () => {
        expect(names(searchItems(items, "pipeline pulse"))).toEqual(["Pipeline Pulse"]);
        expect(searchItems(items, "pipeline nonsense")).toEqual([]);
    });
    it("maps everyday words to categories", () => {
        expect(names(searchItems(items, "calendar"))).toEqual(["Calendly"]);
        expect(names(searchItems(items, "chat"))).toEqual(expect.arrayContaining(["Slack", "Reply Radar"]));
    });
    it("marks the matched words", () => {
        expect(highlightParts("HubSpot CRM", "hub")).toEqual([
            { text: "Hub", hit: true },
            { text: "Spot CRM", hit: false },
        ]);
    });
});

describe("store filters and sorting", () => {
    it("sorts official apps by rank before community apps by installs", () => {
        expect(names(applyFilters(items, DEFAULT_FILTERS, none)).slice(0, 2)).toEqual(["HubSpot", "Slack"]);
        expect(names(applyFilters(items, { ...DEFAULT_FILTERS, type: "community" }, none))).toEqual(["Reply Radar", "Pipeline Pulse"]);
    });
    it("filters by category, method, featured and availability", () => {
        const f = DEFAULT_FILTERS;
        expect(names(applyFilters(items, { ...f, categories: ["crm"], type: "official" }, none))).toEqual(["HubSpot", "Salesforce", "Close"]);
        expect(names(applyFilters(items, { ...f, methods: ["api_key"] }, none))).toEqual(["Close"]);
        expect(names(applyFilters(items, { ...f, featured: true }, none))).toEqual(["Pipeline Pulse"]);
        expect(names(applyFilters(items, { ...f, hideSoon: true }, none))).not.toContain("Salesforce");
        expect(names(applyFilters(items, { ...f, status: "connected" }, (i) => i.name === "Slack"))).toEqual(["Slack"]);
    });
    it("sorts by name and by newest", () => {
        expect(names(applyFilters(items, { ...DEFAULT_FILTERS, sort: "name" }, none))[0]).toBe("Calendly");
        expect(names(applyFilters(items, { ...DEFAULT_FILTERS, sort: "newest" }, none))[0]).toBe("Reply Radar");
    });
});

describe("filters in the URL", () => {
    it("round-trips and leaves defaults out", () => {
        const f = {
            ...DEFAULT_FILTERS,
            q: "crm",
            sort: "relevance" as const,
            categories: ["crm", "meetings"],
            methods: ["oauth" as const],
            featured: true,
            view: "list" as const,
        };
        const sp = writeFilters(f);
        expect(sp.get("sort")).toBeNull();
        expect(readFilters(sp)).toEqual(f);
        expect(writeFilters({ ...f, sort: "popular" }).get("sort")).toBe("popular");
        expect(writeFilters(DEFAULT_FILTERS).toString()).toBe("");
    });
    it("ignores values it does not know", () => {
        const f = readFilters(new URLSearchParams("sort=bogus&category=crm,games&type=x&method=ftp"));
        expect(f.sort).toBe("popular");
        expect(f.categories).toEqual(["crm"]);
        expect(f.type).toBe("all");
        expect(f.methods).toEqual([]);
    });
});

describe("typing in the store search", () => {
    it("follows the query with best match unless a sort was picked", () => {
        expect(withQuery(new URLSearchParams("category=crm"), "hub").toString()).toBe("q=hub&category=crm");
        expect(withQuery(new URLSearchParams("q=hub"), "").toString()).toBe("");
        expect(withQuery(new URLSearchParams("sort=name"), "hub").get("sort")).toBe("name");
        expect(withQuery(new URLSearchParams("q=hub&sort=relevance"), "").get("sort")).toBeNull();
    });
});
