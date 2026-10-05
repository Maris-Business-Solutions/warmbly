// Salesforce settings page: everything about one Salesforce connection in one
// place. Overview (health, API usage, sync counts, permission check), the sync
// rules and field map (one settings document, one save bar), imports from list
// views and Salesforce campaigns, and the activity log with retry.

"use client";

import React from "react";
import { Link, useBlocker, useParams } from "@tanstack/react-router";
import { motion } from "framer-motion";
import {
    ActivityIcon,
    AlertTriangleIcon,
    ArrowLeftIcon,
    DownloadCloudIcon,
    ExternalLinkIcon,
    GaugeIcon,
    Loader2Icon,
    RefreshCwIcon,
    SaveIcon,
    SlidersHorizontalIcon,
    TableIcon,
    type LucideIcon,
} from "lucide-react";
import toast from "react-hot-toast";

import ScrollStrip from "@/components/ui/scroll-strip";
import ResourceViewers from "@/components/app/presence/ResourceViewers";
import { useConfirm } from "@/hooks/context/confirm";
import { useSearchParams } from "@/hooks/useSearchParams";
import { usePresenceResource } from "@/hooks/PresenceProvider";
import {
    useFinishIntegrationOAuth,
    useReauthIntegration,
} from "@/lib/api/hooks/app/integrations/useIntegrationOAuth";
import {
    useSalesforceImportSources,
    useSalesforceOverview,
    useSalesforceSettings,
    useSalesforceSyncNow,
    useUpdateSalesforceSettings,
} from "@/lib/api/hooks/app/integrations/useSalesforce";
import type { SalesforceSettings } from "@/lib/api/models/app/integrations/Salesforce";
import { authorizeInPopup } from "@/lib/integrations/oauthPopup";
import { cn } from "@/lib/utils";

import ProviderGlyph from "../../_components/ProviderGlyph";
import StatusPill, { HealthDot } from "../../_components/StatusPill";
import ActivityLogTab from "../_components/ActivityLogTab";
import FieldMappingTab from "../_components/FieldMappingTab";
import ImportTab from "../_components/ImportTab";
import OverviewTab from "../_components/OverviewTab";
import { Pill, primaryBtn, secondaryBtn } from "../_components/shared";
import SyncRulesTab from "../_components/SyncRulesTab";
import { errMsg } from "../_components/util";

type TabId = "overview" | "rules" | "fields" | "import" | "activity";

const TABS: { id: TabId; label: string; icon: LucideIcon }[] = [
    { id: "overview", label: "Overview", icon: GaugeIcon },
    { id: "rules", label: "Sync rules", icon: SlidersHorizontalIcon },
    { id: "fields", label: "Field mapping", icon: TableIcon },
    { id: "import", label: "Import", icon: DownloadCloudIcon },
    { id: "activity", label: "Activity log", icon: ActivityIcon },
];

const isTab = (v: string | null): v is TabId => !!v && TABS.some((t) => t.id === v);

export default function SalesforcePage() {
    const { id } = useParams({ from: "/app/integrations/salesforce/$id" });
    const [params, setParams] = useSearchParams();
    const tab: TabId = isTab(params.get("tab")) ? (params.get("tab") as TabId) : "overview";
    const confirm = useConfirm();

    const overview = useSalesforceOverview(id);
    const settingsQ = useSalesforceSettings(id);
    const sources = useSalesforceImportSources(id);
    const update = useUpdateSalesforceSettings(id);
    const syncNow = useSalesforceSyncNow(id);
    const reauth = useReauthIntegration();
    const finishOAuth = useFinishIntegrationOAuth();
    const [reconnecting, setReconnecting] = React.useState(false);

    usePresenceResource(id ? `integration_connection:${id}` : null, "editing");

    // One draft for Sync rules and Field mapping: they are one settings document.
    // `base` is the server copy the draft started from, so a teammate's save is
    // adopted while nobody is editing and never clobbers an edit in progress.
    const saved = settingsQ.data?.settings;
    const [base, setBase] = React.useState<SalesforceSettings | null>(null);
    const [draft, setDraft] = React.useState<SalesforceSettings | null>(null);
    const [saveError, setSaveError] = React.useState<string | null>(null);
    const dirty = !!draft && !!base && JSON.stringify(draft) !== JSON.stringify(base);

    const dirtyRef = React.useRef(dirty);
    dirtyRef.current = dirty;
    React.useEffect(() => {
        if (!saved || dirtyRef.current) return;
        setBase(saved);
        setDraft(structuredClone(saved));
    }, [saved]);

    const patch = React.useCallback((fn: (s: SalesforceSettings) => SalesforceSettings) => {
        setDraft((d) => (d ? fn(d) : d));
        setSaveError(null);
    }, []);

    async function save(next?: SalesforceSettings) {
        const body = next ?? draft;
        if (!body) return;
        setSaveError(null);
        try {
            const res = await update.mutateAsync({ connectionId: id, settings: body });
            setBase(res.settings);
            setDraft(structuredClone(res.settings));
            toast.success("Salesforce settings saved");
        } catch (err) {
            const m = errMsg(err, "Could not save the settings");
            setSaveError(m);
            toast.error(m);
            throw err;
        }
    }

    function discard() {
        if (saved) {
            setBase(saved);
            setDraft(structuredClone(saved));
        }
        setSaveError(null);
    }

    function enableSync() {
        const from = dirty && draft ? draft : saved;
        if (!from) return;
        void save({ ...from, enabled: true }).catch(() => undefined);
    }

    // Leaving with unsaved edits asks first. Tab switches stay on this path.
    const skipGuard = React.useRef(false);
    const blocker = useBlocker({
        shouldBlockFn: React.useCallback(
            ({ current, next }: { current: { pathname: string }; next: { pathname: string } }) =>
                !skipGuard.current && dirtyRef.current && current.pathname !== next.pathname,
            [],
        ),
        withResolver: true,
        enableBeforeUnload: false,
    });
    // The navigation stays held while the dialog is open, so confirming resumes it exactly (Back stays Back).
    React.useEffect(() => {
        if (blocker.status !== "blocked") return;
        confirm.show(
            "You have unsaved Salesforce settings. Leave and discard them?",
            async () => {
                skipGuard.current = true;
                discard();
                blocker.proceed();
            },
            () => blocker.reset(),
        );
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [blocker.status]);
    React.useEffect(() => {
        const handler = (e: BeforeUnloadEvent) => {
            if (dirtyRef.current) {
                e.preventDefault();
                e.returnValue = "";
            }
        };
        window.addEventListener("beforeunload", handler);
        return () => window.removeEventListener("beforeunload", handler);
    }, []);

    function setTab(t: TabId, extra?: Record<string, string>) {
        const next = new URLSearchParams();
        if (t !== "overview") next.set("tab", t);
        for (const [k, v] of Object.entries(extra ?? {})) next.set(k, v);
        setParams(next, { replace: true });
    }

    async function handleReconnect() {
        setReconnecting(true);
        try {
            const { code, state } = await authorizeInPopup(async () => (await reauth.mutateAsync(id)).url);
            await finishOAuth.mutateAsync({ code, state });
            toast.success("Reconnected to Salesforce");
            void overview.refetch();
        } catch (err) {
            toast.error(errMsg(err, "Reconnect failed"));
        } finally {
            setReconnecting(false);
        }
    }

    function runSyncNow() {
        syncNow.mutate(undefined, {
            onSuccess: () => toast.success("Sync started"),
            onError: (err) => toast.error(errMsg(err, "Could not start a sync")),
        });
    }

    const ov = overview.data;
    const instanceUrl = ov?.org.instance_url ?? "";
    const needsReauth = ov?.status === "reauth_required";
    const running = (sources.data ?? []).some((s) => s.status === "running");
    const failed = ov?.counts.failed ?? 0;

    if (overview.isError && !ov) {
        return (
            <div className="flex flex-col min-h-full bg-white">
                <div className="px-3 sm:px-5 pt-3 sm:pt-4 pb-3">
                    <BackLink />
                    <div className="mt-6 rounded-md border border-rose-200 bg-rose-50 px-4 py-3 max-w-xl">
                        <p className="text-[12.5px] font-medium text-rose-800">This Salesforce connection could not be loaded</p>
                        <p className="text-[11.5px] text-rose-700 mt-0.5">
                            {errMsg(overview.error, "It may have been disconnected.")}
                        </p>
                        <button type="button" onClick={() => void overview.refetch()} className={cn(secondaryBtn, "mt-2.5")}>
                            <RefreshCwIcon className="w-3 h-3" />
                            Try again
                        </button>
                    </div>
                </div>
            </div>
        );
    }

    return (
        <div className="flex flex-col min-h-full bg-white">
            <div className="px-3 sm:px-5 pt-3 sm:pt-4 pb-3 flex flex-wrap items-start gap-3">
                <div className="min-w-0 flex-1">
                    <BackLink />
                    <div className="flex items-center gap-2.5 min-w-0">
                        <ProviderGlyph provider="salesforce" name="Salesforce" />
                        <div className="min-w-0">
                            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                                <h1 className="min-w-0 max-w-full text-[18px] font-semibold text-slate-900 truncate">
                                    {ov?.label || "Salesforce"}
                                </h1>
                                {ov && <StatusPill status={ov.status} />}
                                {ov?.org.environment === "sandbox" && <Pill tone="amber">Sandbox</Pill>}
                                <ResourceViewers resource={id ? `integration_connection:${id}` : null} className="shrink-0" />
                            </div>
                            <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[11.5px] text-slate-500 mt-0.5">
                                {ov?.org.account && <span className="truncate">{ov.org.account}</span>}
                                {ov && (
                                    <span className="inline-flex items-center gap-1" title={ov.health_detail || undefined}>
                                        <HealthDot health={ov.health} />
                                        {ov.health}
                                    </span>
                                )}
                                {instanceUrl && (
                                    <span className="font-mono text-[10.5px] text-slate-400 truncate">
                                        {instanceUrl.replace(/^https?:\/\//, "")}
                                    </span>
                                )}
                            </div>
                        </div>
                    </div>
                </div>
                <div className="flex items-center gap-1.5 shrink-0 flex-wrap justify-end">
                    {needsReauth && (
                        <button
                            type="button"
                            onClick={handleReconnect}
                            disabled={reconnecting}
                            className="h-7 px-3 rounded-md bg-amber-500 hover:bg-amber-600 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                        >
                            {reconnecting ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <RefreshCwIcon className="w-3.5 h-3.5" />}
                            Reconnect
                        </button>
                    )}
                    {instanceUrl && (
                        <a href={instanceUrl} target="_blank" rel="noopener noreferrer" className={secondaryBtn}>
                            <ExternalLinkIcon className="w-3 h-3" />
                            Open Salesforce
                        </a>
                    )}
                    <button
                        type="button"
                        onClick={runSyncNow}
                        disabled={syncNow.isPending || needsReauth || !ov}
                        title={needsReauth ? "Reconnect first" : undefined}
                        className={primaryBtn}
                    >
                        {syncNow.isPending ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <RefreshCwIcon className="w-3.5 h-3.5" />}
                        Sync now
                    </button>
                </div>
            </div>

            <div className="shrink-0 border-b border-slate-200 px-3">
                <ScrollStrip activeKey={tab} innerClassName="gap-1">
                    {TABS.map((t) => {
                        const active = tab === t.id;
                        return (
                            <button
                                key={t.id}
                                type="button"
                                data-active={active}
                                onClick={() => setTab(t.id)}
                                className={cn(
                                    "relative h-10 px-2.5 inline-flex items-center gap-1.5 text-[12.5px] outline-none whitespace-nowrap transition-colors shrink-0",
                                    active ? "text-slate-900 font-medium" : "text-slate-500 hover:text-slate-800",
                                )}
                            >
                                <t.icon className="w-3.5 h-3.5" />
                                {t.label}
                                {t.id === "activity" && failed > 0 && (
                                    <span className="h-4 min-w-4 px-1 rounded-full bg-rose-50 text-rose-700 text-[10px] font-semibold tabular-nums inline-flex items-center justify-center">
                                        {failed > 99 ? "99+" : failed}
                                    </span>
                                )}
                                {t.id === "import" && running && <Loader2Icon className="w-3 h-3 animate-spin text-sky-600" />}
                                {(t.id === "rules" || t.id === "fields") && dirty && (
                                    <span className="size-1.5 rounded-full bg-amber-500" aria-label="Unsaved changes" />
                                )}
                                {active && (
                                    <motion.span
                                        layoutId="salesforce-tab-underline"
                                        className="absolute left-1.5 right-1.5 -bottom-px h-0.5 rounded-full bg-sky-600"
                                        transition={{ type: "spring", duration: 0.3, bounce: 0.15 }}
                                    />
                                )}
                            </button>
                        );
                    })}
                </ScrollStrip>
            </div>

            <div className="flex-1 px-3 sm:px-5 py-5 pb-24">
                {tab === "overview" && (
                    <OverviewTab
                        connectionId={id}
                        overview={ov}
                        loading={overview.isPending}
                        onEnable={enableSync}
                        enabling={update.isPending}
                        onOpenActivity={(status) => setTab("activity", status ? { status } : undefined)}
                    />
                )}
                {(tab === "rules" || tab === "fields") && !draft && (
                    <div className="py-10 flex items-center justify-center gap-2 text-[12px] text-slate-400">
                        {settingsQ.isError ? (
                            <span className="text-rose-600">{errMsg(settingsQ.error, "Could not load the settings")}</span>
                        ) : (
                            <>
                                <Loader2Icon className="w-3.5 h-3.5 animate-spin" />
                                Loading settings…
                            </>
                        )}
                    </div>
                )}
                {tab === "rules" && draft && <SyncRulesTab connectionId={id} draft={draft} patch={patch} />}
                {tab === "fields" && draft && (
                    <FieldMappingTab
                        connectionId={id}
                        draft={draft}
                        patch={patch}
                        warmblyFields={settingsQ.data?.warmbly_fields ?? []}
                        defaultFieldMap={settingsQ.data?.defaults?.field_map ?? []}
                    />
                )}
                {tab === "import" && <ImportTab connectionId={id} />}
                {tab === "activity" && (
                    <ActivityLogTab
                        connectionId={id}
                        instanceUrl={instanceUrl}
                        status={params.get("status") ?? ""}
                        onStatus={(s) => setTab("activity", s ? { status: s } : undefined)}
                    />
                )}
            </div>

            {dirty && (
                <motion.div
                    initial={{ y: 12, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                    className="fixed bottom-4 left-1/2 -translate-x-1/2 z-30 max-w-[calc(100vw-16px)] flex flex-wrap items-center justify-center gap-2 rounded-md border border-slate-200 bg-white shadow-[0_6px_20px_-4px_rgba(15,23,42,0.12),0_2px_4px_rgba(15,23,42,0.04)] px-3 py-1.5"
                >
                    {saveError ? (
                        <span className="text-[11.5px] text-rose-700 inline-flex items-center gap-1.5 max-w-[420px]">
                            <AlertTriangleIcon className="w-3 h-3 shrink-0" />
                            <span className="truncate" title={saveError}>{saveError}</span>
                        </span>
                    ) : (
                        <span className="text-[12px] text-slate-600">Unsaved Salesforce settings</span>
                    )}
                    <button type="button" onClick={discard} disabled={update.isPending} className={secondaryBtn}>
                        Discard
                    </button>
                    <button
                        type="button"
                        onClick={() => void save().catch(() => undefined)}
                        disabled={update.isPending}
                        className={primaryBtn}
                    >
                        {update.isPending ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <SaveIcon className="w-3.5 h-3.5" />}
                        Save changes
                    </button>
                </motion.div>
            )}
        </div>
    );
}

function BackLink() {
    return (
        <Link
            to="/app/integrations"
            className="inline-flex items-center gap-1 h-6 -ml-1.5 px-1.5 mb-1 rounded-md text-[11.5px] text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
        >
            <ArrowLeftIcon className="w-3 h-3" />
            Integrations
        </Link>
    );
}
