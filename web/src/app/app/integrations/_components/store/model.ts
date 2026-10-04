// The directory's one item shape over built-in integrations and community apps,
// plus the presentation the store reuses: links, brand colours, categories.

import {
    BellIcon,
    BoxIcon,
    BriefcaseIcon,
    CalendarDaysIcon,
    DatabaseIcon,
    ShieldCheckIcon,
    SparklesIcon,
    WorkflowIcon,
    type LucideIcon,
} from "lucide-react";

import { communityAppPath, LISTING_CATEGORY_LABELS, type CommunityApp } from "@/lib/api/models/app/integrations/Community";
import {
    CATEGORY_LABELS,
    type IntegrationCatalogEntry,
    type IntegrationCategory,
} from "@/lib/api/models/app/integrations/Integration";

export type StoreItem =
    | { kind: "builtin"; key: string; name: string; tagline: string; category: string; entry: IntegrationCatalogEntry }
    | { kind: "community"; key: string; name: string; tagline: string; category: string; app: CommunityApp };

export type StoreCategory = IntegrationCategory | "ai" | "other";

export const STORE_CATEGORIES: StoreCategory[] = ["crm", "notifications", "automation", "meetings", "verification", "data", "ai", "other"];

export const CATEGORY_META: Record<StoreCategory, { icon: LucideIcon; blurb: string }> = {
    crm: { icon: BriefcaseIcon, blurb: "Send replies and leads to your pipeline" },
    notifications: { icon: BellIcon, blurb: "Hear about replies the moment they land" },
    automation: { icon: WorkflowIcon, blurb: "Wire Warmbly into thousands of apps" },
    meetings: { icon: CalendarDaysIcon, blurb: "Track booked calls on the contact" },
    verification: { icon: ShieldCheckIcon, blurb: "Check addresses before you send" },
    data: { icon: DatabaseIcon, blurb: "Enrich and move contact data" },
    ai: { icon: SparklesIcon, blurb: "Drafts, research and triage" },
    other: { icon: BoxIcon, blurb: "Everything else" },
};

export function categoryLabel(c: string): string {
    return LISTING_CATEGORY_LABELS[c as StoreCategory] ?? CATEGORY_LABELS[c as IntegrationCategory] ?? c;
}

export function builtinItem(entry: IntegrationCatalogEntry): StoreItem {
    return { kind: "builtin", key: `b:${entry.provider}`, name: entry.name, tagline: entry.tagline, category: entry.category, entry };
}

export function communityItem(app: CommunityApp): StoreItem {
    return { kind: "community", key: `c:${app.slug}`, name: app.name, tagline: app.tagline, category: app.category, app };
}

export const STORE_BASE = "/app/integrations";

export function itemPath(item: StoreItem): string {
    return item.kind === "builtin" ? `${STORE_BASE}/${item.entry.provider}` : communityAppPath(item.app.slug);
}

export function isUsable(entry: IntegrationCatalogEntry): boolean {
    return entry.auth_method !== "oauth" || entry.configured;
}

export function developerLabel(item: StoreItem): string {
    return item.kind === "builtin" ? "Warmbly" : item.app.developer || "Community developer";
}

export function installsLabel(n: number): string {
    return `${n.toLocaleString()} ${n === 1 ? "workspace" : "workspaces"}`;
}



// --- search, sort and filters --------------------------------------------------

// Everyday words that mean a category, so "chat" finds Slack and "calendar" finds meetings.
const SYNONYMS: Record<string, StoreCategory[]> = {
    crm: ["crm"],
    pipeline: ["crm"],
    deals: ["crm"],
    sales: ["crm"],
    chat: ["notifications"],
    alert: ["notifications"],
    alerts: ["notifications"],
    notify: ["notifications"],
    message: ["notifications"],
    zap: ["automation"],
    workflow: ["automation"],
    workflows: ["automation"],
    nocode: ["automation"],
    calendar: ["meetings"],
    booking: ["meetings"],
    schedule: ["meetings"],
    call: ["meetings"],
    calls: ["meetings"],
    verify: ["verification"],
    validate: ["verification"],
    bounce: ["verification"],
    bounces: ["verification"],
    enrich: ["data"],
    enrichment: ["data"],
    export: ["data"],
    sheets: ["data"],
    gpt: ["ai"],
    llm: ["ai"],
    assistant: ["ai"],
};

function tokens(q: string): string[] {
    return q.toLowerCase().split(/\s+/).map((t) => t.trim()).filter(Boolean);
}

function haystack(item: StoreItem) {
    return {
        name: item.name.toLowerCase(),
        tagline: item.tagline.toLowerCase(),
        category: `${item.category} ${categoryLabel(item.category)}`.toLowerCase(),
        body: (item.kind === "builtin"
            ? `${(item.entry.highlights ?? []).join(" ")} ${item.entry.provider}`
            : `${item.app.description} ${item.app.slug}`
        ).toLowerCase(),
        maker: developerLabel(item).toLowerCase(),
    };
}

// Relevance of an item to a query; 0 means it does not match. Every word has to
// land somewhere, and a word in the name counts most.
export function searchScore(item: StoreItem, q: string): number {
    const words = tokens(q);
    if (words.length === 0) return 1;
    const h = haystack(item);
    let total = 0;
    for (const w of words) {
        let best = 0;
        if (h.name.startsWith(w)) best = 10;
        else if (h.name.split(/[^a-z0-9]+/).some((part) => part.startsWith(w))) best = 8;
        else if (h.name.includes(w)) best = 6;
        else if (h.category.includes(w) || SYNONYMS[w]?.includes(item.category as StoreCategory)) best = 5;
        else if (h.tagline.includes(w)) best = 4;
        else if (h.body.includes(w) || h.maker.includes(w)) best = 2;
        if (best === 0) return 0;
        total += best;
    }
    if (h.name === q.trim().toLowerCase()) total += 20;
    return total;
}

export function searchItems(items: StoreItem[], q: string): StoreItem[] {
    return items
        .map((it) => ({ it, s: searchScore(it, q) }))
        .filter((x) => x.s > 0)
        .sort((a, b) => b.s - a.s)
        .map((x) => x.it);
}

export type SortKey = "relevance" | "popular" | "installs" | "newest" | "name" | "name_desc";

export const SORT_LABELS: Record<SortKey, string> = {
    relevance: "Best match",
    popular: "Most popular",
    installs: "Most installed",
    newest: "Newest",
    name: "Name, A to Z",
    name_desc: "Name, Z to A",
};

export type TypeFilter = "all" | "official" | "community";
export type StatusFilter = "all" | "connected" | "available";
export type MethodFilter = "oauth" | "api_key" | "webhook";

export const METHOD_LABELS: Record<MethodFilter, string> = {
    oauth: "One-click sign in",
    api_key: "API key",
    webhook: "Webhook URL",
};

export interface StoreFilters {
    q: string;
    sort: SortKey;
    categories: string[];
    type: TypeFilter;
    status: StatusFilter;
    methods: MethodFilter[];
    featured: boolean;
    hideSoon: boolean;
    view: "grid" | "list";
}

export const DEFAULT_FILTERS: StoreFilters = {
    q: "",
    sort: "popular",
    categories: [],
    type: "all",
    status: "all",
    methods: [],
    featured: false,
    hideSoon: false,
    view: "grid",
};

const list = (v: string | null) => (v ? v.split(",").map((x) => x.trim()).filter(Boolean) : []);

export function readFilters(sp: URLSearchParams): StoreFilters {
    const sort = sp.get("sort") as SortKey | null;
    const type = sp.get("type") as TypeFilter | null;
    const status = sp.get("status") as StatusFilter | null;
    return {
        q: sp.get("q") ?? "",
        sort: sort && sort in SORT_LABELS ? sort : sp.get("q") ? "relevance" : "popular",
        categories: list(sp.get("category")).filter((c) => (STORE_CATEGORIES as string[]).includes(c)),
        type: type === "official" || type === "community" ? type : "all",
        status: status === "connected" || status === "available" ? status : "all",
        methods: list(sp.get("method")).filter((m): m is MethodFilter => m in METHOD_LABELS),
        featured: sp.get("featured") === "1",
        hideSoon: sp.get("soon") === "hide",
        view: sp.get("view") === "list" ? "list" : "grid",
    };
}

// Only what differs from the defaults goes in the URL, so a plain link stays plain.
export function writeFilters(f: StoreFilters): URLSearchParams {
    const sp = new URLSearchParams();
    if (f.q.trim()) sp.set("q", f.q);
    if (f.sort !== (f.q.trim() ? "relevance" : "popular")) sp.set("sort", f.sort);
    if (f.categories.length) sp.set("category", f.categories.join(","));
    if (f.type !== "all") sp.set("type", f.type);
    if (f.status !== "all") sp.set("status", f.status);
    if (f.methods.length) sp.set("method", f.methods.join(","));
    if (f.featured) sp.set("featured", "1");
    if (f.hideSoon) sp.set("soon", "hide");
    if (f.view === "list") sp.set("view", "list");
    return sp;
}

// The address after typing in the search box: a sort nobody picked follows the
// query (best match while searching, most popular otherwise).
export function withQuery(sp: URLSearchParams, q: string): URLSearchParams {
    const f = readFilters(sp);
    let sort: SortKey = q.trim() ? "relevance" : "popular";
    if (sp.has("sort")) sort = f.sort === "relevance" && !q.trim() ? "popular" : f.sort;
    return writeFilters({ ...f, q, sort });
}

export function itemMethod(item: StoreItem): MethodFilter {
    // A community app installs through the OAuth consent screen.
    return item.kind === "builtin" ? (item.entry.auth_method as MethodFilter) : "oauth";
}

export function applyFilters(items: StoreItem[], f: StoreFilters, isConnected: (item: StoreItem) => boolean): StoreItem[] {
    let out = f.q.trim() ? searchItems(items, f.q) : items;
    out = out.filter((it) => {
        if (f.categories.length && !f.categories.includes(it.category)) return false;
        if (f.type === "official" && it.kind !== "builtin") return false;
        if (f.type === "community" && it.kind !== "community") return false;
        if (f.status === "connected" && !isConnected(it)) return false;
        if (f.status === "available" && isConnected(it)) return false;
        if (f.methods.length && !f.methods.includes(itemMethod(it))) return false;
        if (f.featured && !(it.kind === "community" && it.app.status === "featured")) return false;
        if (f.hideSoon && it.kind === "builtin" && !isUsable(it.entry)) return false;
        return true;
    });
    return sortItems(out, f.sort, f.q);
}

function popularity(it: StoreItem): number {
    // Official integrations rank by instance usage; community apps by their installs, after them.
    return it.kind === "builtin" ? 1_000_000 - (it.entry.rank || 999) : it.app.installs;
}

export function sortItems(items: StoreItem[], sort: SortKey, q: string): StoreItem[] {
    const byName = (a: StoreItem, b: StoreItem) => a.name.localeCompare(b.name);
    const copy = [...items];
    switch (sort) {
        case "relevance":
            return q.trim() ? copy : copy.sort((a, b) => popularity(b) - popularity(a));
        case "popular":
            return copy.sort((a, b) => popularity(b) - popularity(a) || byName(a, b));
        case "installs":
            return copy.sort((a, b) => {
                const ia = a.kind === "community" ? a.app.installs : -1;
                const ib = b.kind === "community" ? b.app.installs : -1;
                return ib - ia || popularity(b) - popularity(a);
            });
        case "newest":
            return copy.sort((a, b) => {
                const ta = a.kind === "community" ? new Date(a.app.published_at).getTime() : 0;
                const tb = b.kind === "community" ? new Date(b.app.published_at).getTime() : 0;
                return tb - ta || popularity(b) - popularity(a);
            });
        case "name":
            return copy.sort(byName);
        case "name_desc":
            return copy.sort((a, b) => byName(b, a));
    }
}

// Splits text around the query's words so a result can mark what matched.
export function highlightParts(text: string, q: string): { text: string; hit: boolean }[] {
    const words = tokens(q).filter((w) => w.length > 1);
    if (!words.length) return [{ text, hit: false }];
    const escaped = words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
    const re = new RegExp(`(${escaped.join("|")})`, "gi");
    return text
        .split(re)
        .filter((part) => part !== "")
        .map((part) => ({ text: part, hit: words.includes(part.toLowerCase()) }));
}
