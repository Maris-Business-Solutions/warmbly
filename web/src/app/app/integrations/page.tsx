// Integrations directory.
//
// One searchable place for everything that connects to Warmbly: the built-in
// integrations ranked by how many workspaces use them, verified community apps
// other developers published, and the tools to build your own. Recommendations
// fill the gaps in what this workspace has connected. A community app opened by
// its link (/app/integrations/apps/:slug) gets the same drawer, which is the
// only way to reach one that has not been verified yet.

"use client";

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Link, Navigate, useLocation, useNavigate, useParams } from "react-router-dom";
import {
    AlertTriangleIcon,
    ArrowRightIcon,
    BadgeCheckIcon,
    BlocksIcon,
    BotIcon,
    CodeIcon,
    InboxIcon,
    KeyRoundIcon,
    RefreshCwIcon,
    SettingsIcon,
    SparklesIcon,
    WebhookIcon,
} from "lucide-react";

import { EmptyBlock, Page, PageBody, PageTopbar, SectionBar } from "@/components/layout/Page";
import { SearchInput } from "@/components/ui/field";
import ScrollStrip from "@/components/ui/scroll-strip";
import { useCommunityApps } from "@/lib/api/hooks/app/integrations/useCommunityApps";
import useIntegrationCatalog from "@/lib/api/hooks/app/integrations/useIntegrationCatalog";
import useIntegrationConnections from "@/lib/api/hooks/app/integrations/useIntegrationConnections";
import {
    communityAppPath,
    LISTING_CATEGORY_LABELS,
    type CommunityApp,
} from "@/lib/api/models/app/integrations/Community";
import {
    CATEGORY_LABELS,
    type IntegrationCatalogEntry,
    type IntegrationCategory,
    type IntegrationConnection,
    type IntegrationProvider,
} from "@/lib/api/models/app/integrations/Integration";
import { cn } from "@/lib/utils";

import CommunityAppDrawer from "./_components/CommunityAppDrawer";
import CommunityLogo from "./_components/CommunityLogo";
import ConnectDrawer from "./_components/ConnectDrawer";
import ConnectionDetail from "./_components/ConnectionDetail";
import InboundUrlDialog from "./_components/InboundUrlDialog";
import ProviderGlyph from "./_components/ProviderGlyph";
import StatusPill from "./_components/StatusPill";

// --- directory model ---------------------------------------------------------

type Item =
    | { kind: "builtin"; key: string; name: string; tagline: string; category: string; entry: IntegrationCatalogEntry }
    | { kind: "community"; key: string; name: string; tagline: string; category: string; app: CommunityApp };

type Filter = "all" | IntegrationCategory | "ai" | "other" | "community" | "build";
type Sort = "popular" | "name";

const FILTER_ORDER: Filter[] = [
    "all",
    "crm",
    "notifications",
    "automation",
    "meetings",
    "verification",
    "data",
    "ai",
    "other",
    "community",
    "build",
];

function filterLabel(f: Filter): string {
    if (f === "all") return "All";
    if (f === "community") return "Community";
    if (f === "build") return "Build your own";
    return LISTING_CATEGORY_LABELS[f] ?? CATEGORY_LABELS[f as IntegrationCategory] ?? f;
}

// What a recommendation fills, and why it is worth it, in the order they are offered.
const GAPS: { category: IntegrationCategory; prefer?: IntegrationProvider; why: string }[] = [
    { category: "crm", why: "No CRM connected yet. Send positive replies and booked leads to your pipeline automatically." },
    { category: "notifications", prefer: "slack", why: "Get pinged the moment a prospect replies, instead of checking the inbox." },
    { category: "meetings", why: "Credit every booked call to the campaign that earned it." },
    { category: "verification", why: "Check addresses before you send, so bounces don’t cost you sender reputation." },
];

const POPULAR_COUNT = 6;
const COMMUNITY_PREVIEW = 6;

function isUsable(entry: IntegrationCatalogEntry): boolean {
    return entry.auth_method !== "oauth" || entry.configured;
}

function matches(item: Item, q: string): number {
    // 0 = no match; higher is better. Name hits beat description hits.
    const name = item.name.toLowerCase();
    if (name.startsWith(q)) return 4;
    if (name.includes(q)) return 3;
    const hay = [
        item.tagline,
        item.category,
        filterLabel(item.category as Filter),
        item.kind === "builtin" ? (item.entry.highlights ?? []).join(" ") : item.app.description,
        item.kind === "builtin" ? item.entry.provider : item.app.developer,
    ]
        .join(" ")
        .toLowerCase();
    return hay.includes(q) ? 1 : 0;
}

// --- page ---------------------------------------------------------------------

export default function IntegrationsPage() {
    const params = useParams<{ slug?: string }>();
    const { pathname } = useLocation();
    // The route's /apps segment is optional, so a bare /integrations/<x> is not a link.
    if (params.slug && !pathname.startsWith("/app/integrations/apps/")) {
        return <Navigate to="/app/integrations" replace />;
    }
    return <IntegrationsDirectory slug={params.slug} />;
}

function IntegrationsDirectory({ slug }: { slug?: string }) {
    const navigate = useNavigate();

    const catalogQuery = useIntegrationCatalog();
    const connectionsQuery = useIntegrationConnections();
    const communityQuery = useCommunityApps();

    const [connectTarget, setConnectTarget] = React.useState<IntegrationCatalogEntry | null>(null);
    const [manageTarget, setManageTarget] = React.useState<IntegrationConnection | null>(null);
    const [inboundUrl, setInboundUrl] = React.useState<{ provider: IntegrationProvider; url: string } | null>(null);
    const [query, setQuery] = React.useState("");
    const [filter, setFilter] = React.useState<Filter>("all");
    const [sort, setSort] = React.useState<Sort>("popular");

    const catalog = React.useMemo(() => catalogQuery.data?.catalog ?? [], [catalogQuery.data?.catalog]);
    const connections = React.useMemo(
        () => connectionsQuery.data?.connections ?? [],
        [connectionsQuery.data?.connections],
    );
    const community = React.useMemo(() => communityQuery.data?.data ?? [], [communityQuery.data?.data]);

    const entryByProvider = React.useMemo(() => {
        const m: Partial<Record<IntegrationProvider, IntegrationCatalogEntry>> = {};
        for (const e of catalog) m[e.provider] = e;
        return m;
    }, [catalog]);

    const firstConnByProvider = React.useMemo(() => {
        const m: Partial<Record<IntegrationProvider, IntegrationConnection>> = {};
        for (const c of connections) if (!m[c.provider]) m[c.provider] = c;
        return m;
    }, [connections]);

    const byRank = React.useMemo(() => [...catalog].sort((a, b) => (a.rank || 99) - (b.rank || 99)), [catalog]);

    const items = React.useMemo<Item[]>(
        () => [
            ...byRank.map<Item>((entry) => ({
                kind: "builtin",
                key: `b:${entry.provider}`,
                name: entry.name,
                tagline: entry.tagline,
                category: entry.category,
                entry,
            })),
            ...community.map<Item>((app) => ({
                kind: "community",
                key: `c:${app.slug}`,
                name: app.name,
                tagline: app.tagline,
                category: app.category,
                app,
            })),
        ],
        [byRank, community],
    );

    const counts = React.useMemo(() => {
        const m: Partial<Record<Filter, number>> = { all: items.length, community: community.length, build: BUILD_TOOLS.length };
        for (const it of items) m[it.category as Filter] = (m[it.category as Filter] ?? 0) + 1;
        return m;
    }, [items, community.length]);

    const visibleFilters = FILTER_ORDER.filter((f) => f === "all" || f === "build" || f === "community" || (counts[f] ?? 0) > 0);

    const recommendations = React.useMemo(() => {
        const connectedCategories = new Set(
            connections
                .filter((c) => c.status !== "disconnected")
                .map((c) => entryByProvider[c.provider]?.category)
                .filter(Boolean),
        );
        const out: { entry: IntegrationCatalogEntry; why: string }[] = [];
        for (const gap of GAPS) {
            if (connectedCategories.has(gap.category)) continue;
            const candidates = byRank.filter((e) => e.category === gap.category && isUsable(e));
            const pick = candidates.find((e) => e.provider === gap.prefer) ?? candidates[0];
            if (pick) out.push({ entry: pick, why: gap.why });
            if (out.length === 3) break;
        }
        return out;
    }, [connections, entryByProvider, byRank]);

    const q = query.trim().toLowerCase();
    const browsing = filter === "all" && !q;

    const results = React.useMemo(() => {
        if (browsing || filter === "build") return [];
        let list = items.filter((it) =>
            filter === "all" ? true : filter === "community" ? it.kind === "community" : it.category === filter,
        );
        const score = new Map<string, number>();
        if (q) {
            list = list.filter((it) => {
                const s = matches(it, q);
                score.set(it.key, s);
                return s > 0;
            });
        }
        // Built-ins carry an instance rank; community apps follow, most installed first.
        const popularity = (it: Item) => (it.kind === "builtin" ? it.entry.rank || 999 : 1_000_000 - it.app.installs);
        return [...list].sort((a, b) => {
            const s = (score.get(b.key) ?? 0) - (score.get(a.key) ?? 0);
            if (s !== 0) return s;
            if (sort === "name") return a.name.localeCompare(b.name);
            return popularity(a) - popularity(b);
        });
    }, [browsing, filter, items, q, sort]);

    const attention = connections.filter((c) => c.status === "degraded" || c.status === "reauth_required");
    const popular = byRank.filter((e) => !firstConnByProvider[e.provider] && isUsable(e)).slice(0, POPULAR_COUNT);
    const recommendedProviders = new Set(recommendations.map((r) => r.entry.provider));

    function refreshAll() {
        void catalogQuery.refetch();
        void connectionsQuery.refetch();
        void communityQuery.refetch();
    }

    function openItem(it: Item) {
        if (it.kind === "community") {
            navigate(communityAppPath(it.app.slug));
            return;
        }
        const existing = firstConnByProvider[it.entry.provider];
        if (existing) setManageTarget(existing);
        else setConnectTarget(it.entry);
    }

    function openEntry(entry: IntegrationCatalogEntry) {
        openItem({ kind: "builtin", key: entry.provider, name: entry.name, tagline: entry.tagline, category: entry.category, entry });
    }

    const topRef = React.useRef<HTMLDivElement | null>(null);
    function pickFilter(f: Filter) {
        setFilter(f);
        topRef.current?.scrollIntoView({ block: "nearest" });
    }

    const loading = catalogQuery.isPending;
    const previewApp = slug ? community.find((a) => a.slug === slug) : undefined;

    return (
        <Page>
            <PageTopbar eyebrow="Integrations" subtitle="Connect the tools your team already uses">
                <button
                    type="button"
                    onClick={() => pickFilter("build")}
                    className="hidden sm:inline-flex h-7 px-2.5 rounded-md text-[12px] text-slate-600 hover:text-slate-900 hover:bg-slate-100 items-center gap-1.5 transition-colors"
                >
                    <CodeIcon className="w-3.5 h-3.5" />
                    Build your own
                </button>
                <button
                    type="button"
                    onClick={refreshAll}
                    aria-label="Refresh"
                    className="h-7 w-7 rounded-md border border-slate-200 hover:border-slate-300 text-slate-500 hover:text-slate-900 inline-flex items-center justify-center transition-colors"
                >
                    <RefreshCwIcon
                        className={cn("w-3 h-3", (connectionsQuery.isFetching || communityQuery.isFetching) && "animate-spin")}
                    />
                </button>
            </PageTopbar>

            {/* Search + filters */}
            <div className="border-b border-slate-200 bg-gradient-to-b from-slate-50/80 to-white shrink-0">
                <div className="px-5 pt-5 pb-3 max-w-3xl">
                    <h1 className="text-[17px] font-semibold text-slate-900 tracking-tight">Find an integration</h1>
                    <p className="mt-0.5 text-[12.5px] text-slate-500">
                        {catalog.length} built-in
                        {community.length > 0 && ` and ${community.length} community`} integrations, most used first.
                    </p>
                    <SearchInput
                        value={query}
                        onChange={setQuery}
                        placeholder="Search by name, use or category"
                        className="mt-3 h-9 max-w-xl"
                    />
                </div>
                <div className="px-5 pb-2.5 flex items-center gap-3">
                    <ScrollStrip activeKey={filter} className="flex-1 min-w-0" innerClassName="gap-1.5" fade="from-white">
                        {visibleFilters.map((f) => (
                            <button
                                key={f}
                                type="button"
                                data-active={filter === f ? "true" : undefined}
                                onClick={() => pickFilter(f)}
                                className={cn(
                                    "h-7 px-2.5 rounded-full border text-[12px] whitespace-nowrap inline-flex items-center gap-1.5 transition-colors",
                                    filter === f
                                        ? "border-sky-300 bg-sky-50 text-sky-800 font-medium"
                                        : "border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:text-slate-900",
                                )}
                            >
                                {f === "build" && <CodeIcon className="w-3 h-3" />}
                                {f === "community" && <BadgeCheckIcon className="w-3 h-3" />}
                                {filterLabel(f)}
                                {f !== "all" && f !== "build" && (
                                    <span className="font-mono text-[10px] text-slate-400 tabular-nums">{counts[f] ?? 0}</span>
                                )}
                            </button>
                        ))}
                    </ScrollStrip>
                    {!browsing && filter !== "build" && (
                        <div className="shrink-0 hidden sm:flex items-center rounded-md border border-slate-200 p-0.5">
                            {(["popular", "name"] as const).map((s) => (
                                <button
                                    key={s}
                                    type="button"
                                    onClick={() => setSort(s)}
                                    className={cn(
                                        "h-6 px-2 rounded text-[11.5px] transition-colors",
                                        sort === s ? "bg-slate-100 text-slate-900 font-medium" : "text-slate-500 hover:text-slate-800",
                                    )}
                                >
                                    {s === "popular" ? "Popular" : "A to Z"}
                                </button>
                            ))}
                        </div>
                    )}
                </div>
            </div>

            <PageBody>
                <div ref={topRef} />
                <div>
                    <AnimatePresence mode="wait" initial={false}>
                        <motion.div
                            key={browsing ? "browse" : filter === "build" ? "build" : "results"}
                            initial={{ opacity: 0, y: 4 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0, y: -4 }}
                            transition={{ duration: 0.14, ease: "easeOut" }}
                        >
                            {loading ? (
                                <SkeletonGrid />
                            ) : filter === "build" ? (
                                <BuildSection />
                            ) : !browsing ? (
                                <ResultsView
                                    results={results}
                                    query={query}
                                    filter={filter}
                                    connections={firstConnByProvider}
                                    onOpen={openItem}
                                    onBuild={() => pickFilter("build")}
                                    onClear={() => {
                                        setQuery("");
                                        setFilter("all");
                                    }}
                                />
                            ) : (
                                <>
                                    {attention.length > 0 && (
                                        <div className="mx-5 mt-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 flex items-center gap-2 flex-wrap">
                                            <AlertTriangleIcon className="w-3.5 h-3.5 text-amber-600 shrink-0" />
                                            <span className="text-[12px] text-amber-900">
                                                {attention.length === 1
                                                    ? "1 connection needs attention"
                                                    : `${attention.length} connections need attention`}
                                            </span>
                                            <div className="flex items-center gap-1.5 flex-wrap">
                                                {attention.map((c) => (
                                                    <button
                                                        key={c.id}
                                                        type="button"
                                                        onClick={() => setManageTarget(c)}
                                                        className="h-6 px-2 rounded border border-amber-300 bg-white text-[11.5px] text-amber-900 hover:bg-amber-100 transition-colors"
                                                    >
                                                        {c.label || entryByProvider[c.provider]?.name || c.provider}
                                                    </button>
                                                ))}
                                            </div>
                                        </div>
                                    )}

                                    {connections.length > 0 && (
                                        <section className="mt-2">
                                            <SectionBar label="Connected" count={connections.length} />
                                            <div className="grid sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-px bg-slate-200/60 border-b border-slate-200/60">
                                                {connections.map((c, i) => (
                                                    <ConnectionCard
                                                        key={c.id}
                                                        index={i}
                                                        connection={c}
                                                        entry={entryByProvider[c.provider]}
                                                        onManage={() => setManageTarget(c)}
                                                    />
                                                ))}
                                            </div>
                                        </section>
                                    )}

                                    {recommendations.length > 0 && (
                                        <section>
                                            <SectionBar label="Recommended for you">
                                                <SparklesIcon className="w-3 h-3 text-sky-500" />
                                            </SectionBar>
                                            <div className="grid md:grid-cols-3 gap-px bg-slate-200/60 border-b border-slate-200/60">
                                                {recommendations.map((r, i) => (
                                                    <RecommendationCard
                                                        key={r.entry.provider}
                                                        index={i}
                                                        entry={r.entry}
                                                        why={r.why}
                                                        onOpen={() => openEntry(r.entry)}
                                                    />
                                                ))}
                                            </div>
                                        </section>
                                    )}

                                    {popular.length > 0 && (
                                        <section>
                                            <SectionBar label="Popular" />
                                            <CardGrid>
                                                {popular.map((entry, i) => (
                                                    <BuiltinCard
                                                        key={entry.provider}
                                                        index={i}
                                                        entry={entry}
                                                        recommended={recommendedProviders.has(entry.provider)}
                                                        onOpen={() => openEntry(entry)}
                                                    />
                                                ))}
                                            </CardGrid>
                                        </section>
                                    )}

                                    <section>
                                        <SectionBar label="Community" count={community.length || undefined}>
                                            {community.length > COMMUNITY_PREVIEW && (
                                                <button
                                                    type="button"
                                                    onClick={() => pickFilter("community")}
                                                    className="text-[11.5px] text-sky-700 hover:text-sky-800 inline-flex items-center gap-1"
                                                >
                                                    See all
                                                    <ArrowRightIcon className="w-3 h-3" />
                                                </button>
                                            )}
                                        </SectionBar>
                                        {community.length === 0 ? (
                                            <CommunityEmpty onBuild={() => pickFilter("build")} />
                                        ) : (
                                            <CardGrid>
                                                {community.slice(0, COMMUNITY_PREVIEW).map((app, i) => (
                                                    <CommunityCard
                                                        key={app.slug}
                                                        index={i}
                                                        app={app}
                                                        onOpen={() => navigate(communityAppPath(app.slug))}
                                                    />
                                                ))}
                                            </CardGrid>
                                        )}
                                    </section>

                                    <section>
                                        <SectionBar label="All integrations" count={catalog.length} />
                                        <CardGrid>
                                            {byRank.map((entry, i) => (
                                                <BuiltinCard
                                                    key={entry.provider}
                                                    index={i}
                                                    entry={entry}
                                                    connection={firstConnByProvider[entry.provider]}
                                                    showCategory
                                                    onOpen={() => openEntry(entry)}
                                                />
                                            ))}
                                        </CardGrid>
                                    </section>

                                    <BuildSection />
                                </>
                            )}
                        </motion.div>
                    </AnimatePresence>
                </div>
            </PageBody>

            <AnimatePresence>
                {slug && (
                    <CommunityAppDrawer
                        key={slug}
                        slug={slug}
                        preview={previewApp}
                        onClose={() => navigate("/app/integrations")}
                    />
                )}
            </AnimatePresence>
            {connectTarget && (
                <ConnectDrawer
                    entry={connectTarget}
                    onClose={() => setConnectTarget(null)}
                    onConnected={(conn) => {
                        void connectionsQuery.refetch();
                        if (conn.inbound_webhook_url) {
                            setInboundUrl({ provider: conn.provider, url: conn.inbound_webhook_url });
                        } else {
                            // Drop straight into management so the user can wire automations.
                            setManageTarget(conn);
                        }
                    }}
                />
            )}
            {manageTarget && (
                <ConnectionDetail
                    connection={manageTarget}
                    entry={entryByProvider[manageTarget.provider]}
                    onClose={() => {
                        setManageTarget(null);
                        void connectionsQuery.refetch();
                    }}
                />
            )}
            {inboundUrl && (
                <InboundUrlDialog
                    provider={inboundUrl.provider}
                    url={inboundUrl.url}
                    onClose={() => setInboundUrl(null)}
                />
            )}
        </Page>
    );
}

// --- results ------------------------------------------------------------------

function ResultsView({
    results,
    query,
    filter,
    connections,
    onOpen,
    onBuild,
    onClear,
}: {
    results: Item[];
    query: string;
    filter: Filter;
    connections: Partial<Record<IntegrationProvider, IntegrationConnection>>;
    onOpen: (it: Item) => void;
    onBuild: () => void;
    onClear: () => void;
}) {
    const label = query.trim()
        ? filter === "all"
            ? `Results for “${query.trim()}”`
            : `${filterLabel(filter)} matching “${query.trim()}”`
        : filterLabel(filter);

    if (results.length === 0) {
        return (
            <>
                <SectionBar label={label} count={0} />
                {filter === "community" && !query.trim() ? (
                    <CommunityEmpty onBuild={onBuild} />
                ) : (
                    <EmptyBlock
                        title="Nothing matches"
                        body="Connect it through Zapier, Make or n8n, or build your own integration on the Warmbly API."
                        cta={
                            <>
                                <button
                                    type="button"
                                    onClick={onClear}
                                    className="h-7 px-3 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-slate-300"
                                >
                                    Clear search
                                </button>
                                <button
                                    type="button"
                                    onClick={onBuild}
                                    className="h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium"
                                >
                                    Build your own
                                </button>
                            </>
                        }
                    />
                )}
            </>
        );
    }

    return (
        <>
            <SectionBar label={label} count={results.length} />
            <CardGrid>
                {results.map((it, i) =>
                    it.kind === "builtin" ? (
                        <BuiltinCard
                            key={it.key}
                            index={i}
                            entry={it.entry}
                            connection={connections[it.entry.provider]}
                            showCategory={filter === "all"}
                            onOpen={() => onOpen(it)}
                        />
                    ) : (
                        <CommunityCard key={it.key} index={i} app={it.app} onOpen={() => onOpen(it)} />
                    ),
                )}
            </CardGrid>
            {filter === "community" && (
                <p className="px-5 py-4 text-[11.5px] text-slate-400 leading-relaxed max-w-2xl">
                    Only verified apps are listed here. An app that is waiting for review can still be opened from the link its
                    developer shares, and it carries an unverified warning.
                </p>
            )}
        </>
    );
}

// --- cards --------------------------------------------------------------------

function CardGrid({ children }: { children: React.ReactNode }) {
    return (
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-px bg-slate-200/60 border-b border-slate-200/60">{children}</div>
    );
}

const cardMotion = (index: number) => ({
    initial: { opacity: 0, y: 8 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.25, ease: [0.16, 1, 0.3, 1] as const, delay: Math.min(index, 12) * 0.025 },
});

function authLabel(entry: IntegrationCatalogEntry): string {
    if (entry.auth_method === "oauth") return "One-click";
    if (entry.auth_method === "api_key") return "API key";
    return "Webhook";
}

function BuiltinCard({
    entry,
    connection,
    onOpen,
    index = 0,
    recommended = false,
    showCategory = false,
}: {
    entry: IntegrationCatalogEntry;
    connection?: IntegrationConnection;
    onOpen: () => void;
    index?: number;
    recommended?: boolean;
    showCategory?: boolean;
}) {
    const connected = !!connection;
    const comingSoon = !isUsable(entry) && !connected;
    return (
        <motion.button
            type="button"
            onClick={onOpen}
            {...cardMotion(index)}
            className="text-left bg-white p-4 flex flex-col min-h-[136px] hover:bg-slate-50/60 transition-colors group"
        >
            <div className="flex items-start gap-3">
                <ProviderGlyph provider={entry.provider} name={entry.name} />
                <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                        <span className="text-[13px] font-semibold text-slate-900 truncate">{entry.name}</span>
                        {entry.beta && <Badge tone="amber">Beta</Badge>}
                    </div>
                    <div className="text-[11px] text-slate-400 truncate">
                        {showCategory ? `${CATEGORY_LABELS[entry.category]} · ${authLabel(entry)}` : authLabel(entry)}
                    </div>
                </div>
                {connected ? (
                    <StatusPill status={connection.status} />
                ) : comingSoon ? (
                    <Badge tone="slate">Soon</Badge>
                ) : recommended ? (
                    <Badge tone="sky">For you</Badge>
                ) : entry.rank > 0 && entry.rank <= 3 ? (
                    <Badge tone="emerald">Popular</Badge>
                ) : null}
            </div>
            <p className="mt-2.5 text-[12px] text-slate-600 leading-relaxed line-clamp-2">{entry.tagline}</p>
            <div className="mt-auto pt-3 flex items-center justify-end">
                <span
                    className={cn(
                        "h-7 px-2.5 rounded-md text-[11.5px] font-medium inline-flex items-center gap-1 transition-colors",
                        connected
                            ? "text-slate-600 group-hover:text-slate-900 group-hover:bg-slate-100"
                            : comingSoon
                              ? "text-slate-300"
                              : "text-sky-700 group-hover:bg-sky-50",
                    )}
                >
                    {connected ? (
                        <>
                            <SettingsIcon className="w-3 h-3" />
                            Manage
                        </>
                    ) : comingSoon ? (
                        "Coming soon"
                    ) : (
                        <>
                            Connect
                            <ArrowRightIcon className="w-3 h-3 transition-transform group-hover:translate-x-0.5" />
                        </>
                    )}
                </span>
            </div>
        </motion.button>
    );
}

function CommunityCard({ app, onOpen, index = 0 }: { app: CommunityApp; onOpen: () => void; index?: number }) {
    return (
        <motion.button
            type="button"
            onClick={onOpen}
            {...cardMotion(index)}
            className="text-left bg-white p-4 flex flex-col min-h-[136px] hover:bg-slate-50/60 transition-colors group"
        >
            <div className="flex items-start gap-3">
                <CommunityLogo name={app.name} url={app.logo_url} />
                <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                        <span className="text-[13px] font-semibold text-slate-900 truncate">{app.name}</span>
                        {app.verification === "verified" && (
                            <BadgeCheckIcon className="w-3.5 h-3.5 text-sky-600 shrink-0" aria-label="Verified" />
                        )}
                    </div>
                    <div className="text-[11px] text-slate-400 truncate">by {app.developer || "a community developer"}</div>
                </div>
                {app.installed && <Badge tone="emerald">Installed</Badge>}
            </div>
            <p className="mt-2.5 text-[12px] text-slate-600 leading-relaxed line-clamp-2">{app.tagline}</p>
            <div className="mt-auto pt-3 flex items-center justify-between gap-2">
                <span className="text-[11px] text-slate-400 tabular-nums">
                    {app.installs > 0 ? `${app.installs.toLocaleString()} ${app.installs === 1 ? "workspace" : "workspaces"}` : "New"}
                    {" · "}
                    {LISTING_CATEGORY_LABELS[app.category] ?? app.category}
                </span>
                <span className="h-7 px-2.5 rounded-md text-[11.5px] font-medium inline-flex items-center gap-1 text-sky-700 group-hover:bg-sky-50 transition-colors">
                    View
                    <ArrowRightIcon className="w-3 h-3 transition-transform group-hover:translate-x-0.5" />
                </span>
            </div>
        </motion.button>
    );
}

function RecommendationCard({
    entry,
    why,
    onOpen,
    index = 0,
}: {
    entry: IntegrationCatalogEntry;
    why: string;
    onOpen: () => void;
    index?: number;
}) {
    return (
        <motion.button
            type="button"
            onClick={onOpen}
            {...cardMotion(index)}
            className="text-left bg-white p-4 flex flex-col gap-2.5 hover:bg-sky-50/40 transition-colors group"
        >
            <div className="flex items-center gap-3">
                <ProviderGlyph provider={entry.provider} name={entry.name} />
                <div className="min-w-0 flex-1">
                    <div className="text-[13px] font-semibold text-slate-900 truncate">{entry.name}</div>
                    <div className="text-[11px] text-slate-400">{CATEGORY_LABELS[entry.category]}</div>
                </div>
            </div>
            <p className="text-[12px] text-slate-600 leading-relaxed">{why}</p>
            <span className="mt-auto self-start h-7 px-2.5 rounded-md bg-sky-600 group-hover:bg-sky-700 text-white text-[11.5px] font-medium inline-flex items-center gap-1 transition-colors">
                Connect {entry.name}
                <ArrowRightIcon className="w-3 h-3" />
            </span>
        </motion.button>
    );
}

function ConnectionCard({
    connection,
    entry,
    onManage,
    index = 0,
}: {
    connection: IntegrationConnection;
    entry?: IntegrationCatalogEntry;
    onManage: () => void;
    index?: number;
}) {
    const fields = connection.display_fields as Record<string, string> | undefined;
    const account = connection.external_account_name || fields?.account || fields?.workspace || fields?.channel || "";
    return (
        <motion.button
            type="button"
            onClick={onManage}
            {...cardMotion(index)}
            className="text-left bg-white px-4 py-3 flex items-center gap-3 hover:bg-slate-50/60 transition-colors"
        >
            <ProviderGlyph provider={connection.provider} name={entry?.name ?? connection.label} size={7} />
            <div className="min-w-0 flex-1">
                <div className="text-[12.5px] font-semibold text-slate-900 truncate">{connection.label || entry?.name}</div>
                <div className="text-[11px] text-slate-400 truncate">{account || (entry?.name ?? connection.provider)}</div>
            </div>
            <StatusPill status={connection.status} />
        </motion.button>
    );
}

function Badge({ tone, children }: { tone: "sky" | "emerald" | "amber" | "slate"; children: React.ReactNode }) {
    const tones = {
        sky: "bg-sky-50 text-sky-700",
        emerald: "bg-emerald-50 text-emerald-700",
        amber: "bg-amber-50 text-amber-700",
        slate: "bg-slate-100 text-slate-500",
    };
    return (
        <span
            className={cn(
                "inline-flex items-center h-5 px-1.5 rounded text-[9.5px] uppercase tracking-[0.08em] font-medium shrink-0",
                tones[tone],
            )}
        >
            {children}
        </span>
    );
}

function CommunityEmpty({ onBuild }: { onBuild: () => void }) {
    return (
        <div className="px-5 py-6 border-b border-slate-200/60 flex items-center gap-4 flex-wrap">
            <div className="w-9 h-9 rounded-md bg-sky-50 ring-1 ring-sky-100 inline-flex items-center justify-center shrink-0">
                <BlocksIcon className="w-4 h-4 text-sky-600" />
            </div>
            <div className="min-w-0 flex-1">
                <p className="text-[12.5px] font-medium text-slate-800">No community apps yet</p>
                <p className="text-[11.5px] text-slate-500 leading-relaxed">
                    Built something on the Warmbly API? Publish it here so other workspaces can install it.
                </p>
            </div>
            <button
                type="button"
                onClick={onBuild}
                className="h-7 px-3 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-slate-300 hover:text-slate-900 transition-colors"
            >
                Publish an app
            </button>
        </div>
    );
}

function SkeletonGrid() {
    return (
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-px bg-slate-200/60 border-y border-slate-200/60 mt-2">
            {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="bg-white p-4 min-h-[136px] animate-pulse">
                    <div className="flex items-center gap-3">
                        <div className="w-9 h-9 rounded-md bg-slate-100" />
                        <div className="space-y-1.5 flex-1">
                            <div className="h-3 w-24 rounded bg-slate-100" />
                            <div className="h-2.5 w-16 rounded bg-slate-100" />
                        </div>
                    </div>
                    <div className="mt-4 h-2.5 w-full rounded bg-slate-100" />
                    <div className="mt-2 h-2.5 w-2/3 rounded bg-slate-100" />
                </div>
            ))}
        </div>
    );
}

// --- build your own -------------------------------------------------------------

const BUILD_TOOLS: { title: string; body: string; to: string; cta: string; icon: React.ElementType }[] = [
    {
        title: "Publish an app",
        body: "Register an OAuth app and list it in the community directory, so any workspace can install it.",
        to: "/app/settings/oauth-apps",
        cta: "OAuth apps",
        icon: BlocksIcon,
    },
    {
        title: "Webhooks",
        body: "Send signed events (replies, bounces, meetings) to any HTTPS endpoint you run.",
        to: "/app/settings/webhooks",
        cta: "Add a webhook",
        icon: WebhookIcon,
    },
    {
        title: "API keys",
        body: "Call the Warmbly API from your own code with a key scoped to exactly what it needs.",
        to: "/app/api-keys",
        cta: "Create a key",
        icon: KeyRoundIcon,
    },
    {
        title: "Inbound lead webhook",
        body: "Give any form or tool a URL that creates contacts and starts an automation.",
        to: "/app/automations",
        cta: "Automations",
        icon: InboxIcon,
    },
    {
        title: "MCP tools",
        body: "Connect an MCP server so the AI assistant can use your own tools, with your approval each time.",
        to: "/app/settings/connections",
        cta: "Connect a server",
        icon: BotIcon,
    },
];

function BuildSection() {
    return (
        <section>
            <SectionBar label="Build your own">
                <a
                    href="https://docs.warmbly.com/api/oauth/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-[11.5px] text-slate-500 hover:text-sky-700 inline-flex items-center gap-1"
                >
                    Developer docs
                    <ArrowRightIcon className="w-3 h-3" />
                </a>
            </SectionBar>
            <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-px bg-slate-200/60 border-b border-slate-200/60">
                {BUILD_TOOLS.map((tool, i) => (
                    <motion.div key={tool.title} {...cardMotion(i)} className="bg-white">
                        <Link to={tool.to} className="h-full p-4 flex flex-col gap-2 hover:bg-slate-50/60 transition-colors group">
                            <div className="flex items-center gap-3">
                                <div className="w-9 h-9 rounded-md bg-slate-50 ring-1 ring-slate-200 inline-flex items-center justify-center shrink-0">
                                    <tool.icon className="w-4 h-4 text-slate-600" />
                                </div>
                                <span className="text-[13px] font-semibold text-slate-900">{tool.title}</span>
                            </div>
                            <p className="text-[12px] text-slate-600 leading-relaxed">{tool.body}</p>
                            <span className="mt-auto pt-1 text-[11.5px] font-medium text-sky-700 inline-flex items-center gap-1">
                                {tool.cta}
                                <ArrowRightIcon className="w-3 h-3 transition-transform group-hover:translate-x-0.5" />
                            </span>
                        </Link>
                    </motion.div>
                ))}
            </div>
        </section>
    );
}
