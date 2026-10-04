// HubSpot mode, after setup: health, the same choices as the wizard as cards
// that save on their own, and the way back to Warmbly's own CRM.

import React from "react";
import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
    AlertTriangleIcon,
    ArrowLeftIcon,
    CopyPlusIcon,
    ExternalLinkIcon,
    Loader2Icon,
    PlugIcon,
    RefreshCwIcon,
} from "lucide-react";
import toast from "react-hot-toast";

import { HubSpotBadge, HubSpotMark, HubSpotSyncedAt } from "@/components/app/crm/HubSpot";
import { Checkbox } from "@/components/ui/checkbox";
import { useConfirm } from "@/hooks/context/confirm";
import useCrmProvider from "@/hooks/useCrmProvider";
import { useAutosave } from "@/hooks/useAutosave";
import { usePermission } from "@/hooks/usePermission";
import updateCrmSettings from "@/lib/api/client/app/crm/provider/updateCrmSettings";
import useCustomFieldKeys from "@/lib/api/hooks/app/contacts/useCustomFieldKeys";
import { useCrmBackfillPreview, useStartCrmBackfill } from "@/lib/api/hooks/app/crm/provider/useCrmBackfill";
import useCrmMetadata from "@/lib/api/hooks/app/crm/provider/useCrmMetadata";
import { useSyncCrmNow } from "@/lib/api/hooks/app/crm/provider/useCrmSyncActions";
import useCrmSyncHealth from "@/lib/api/hooks/app/crm/provider/useCrmSyncHealth";
import useUpdateCrmSettings from "@/lib/api/hooks/app/crm/provider/useUpdateCrmSettings";
import type { CRMBackfillRequest, CRMProviderConfig, CRMSettings } from "@/lib/api/models/app/crm/CRMProvider";
import type { IntegrationCatalogEntry, IntegrationConnection } from "@/lib/api/models/app/integrations/Integration";
import { useAppStore } from "@/stores";
import { cn } from "@/lib/utils";

import SaveStatus from "@/app/app/settings/_components/SaveStatus";
import ConnectionDetail from "@/app/app/integrations/_components/ConnectionDetail";
import {
    ActivityEditor,
    Card,
    ContactsEditor,
    DisplayPropertiesEditor,
    ExitRulesEditor,
    FieldMappingTable,
    GuardsEditor,
    OwnersTable,
    PipelinesEditor,
    ReplyOutcomeEditor,
    SubLabel,
    WarmblyPropertiesEditor,
} from "./editors";
import { useHubSpotOAuth } from "./hooks";
import SyncHealthCard from "./SyncHealthCard";
import { type ConfigPatch, errCode, errMessage, joinList, normalizeConfig, plural, replyOutcomeIssue } from "./shared";

export default function HubSpotSettings({
    settings,
    connection,
    entry,
}: {
    settings: CRMSettings;
    connection?: IntegrationConnection;
    entry?: IntegrationCatalogEntry;
}) {
    const queryClient = useQueryClient();
    const confirm = useConfirm();
    const canManage = usePermission("MANAGE_SETTINGS");
    const { appUrl, needsReconnect } = useCrmProvider();
    const health = useCrmSyncHealth();
    const syncNow = useSyncCrmNow();
    const update = useUpdateCrmSettings();
    const oauth = useHubSpotOAuth();
    const metadata = useCrmMetadata();
    const customKeys = useCustomFieldKeys();
    const [manageOpen, setManageOpen] = React.useState(false);

    // One workspace's settings: hydrate once per workspace, then the save path
    // owns the baseline so a refetch cannot overwrite an edit in flight.
    const orgID = useAppStore((st) => st.currentOrganization?.id);
    const hydratedFor = React.useRef<string | undefined>(undefined);
    const [draft, setDraft] = React.useState<CRMProviderConfig | null>(null);
    const issue = draft ? replyOutcomeIssue(draft) : null;

    const autosave = useAutosave({
        value: draft,
        enabled: !!draft && !issue && canManage,
        debounceMs: 500,
        save: async (v) => {
            if (!v) return;
            if (hydratedFor.current !== useAppStore.getState().currentOrganization?.id) return;
            try {
                const res = await updateCrmSettings({ config: v });
                queryClient.setQueryData(["crm", "settings"], res);
                void queryClient.invalidateQueries({ queryKey: ["crm", "contact"] });
            } catch (err) {
                toast.error(errMessage(err, "Could not save HubSpot settings"));
                throw err;
            }
        },
    });

    React.useEffect(() => {
        if (hydratedFor.current === orgID) return;
        hydratedFor.current = orgID;
        const cfg = normalizeConfig(settings.config);
        setDraft(cfg);
        autosave.markSaved(cfg);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [settings, orgID]);

    const patch: ConfigPatch = React.useCallback((fn) => setDraft((c) => (c ? fn(c) : c)), []);

    function runSync() {
        syncNow.mutate(undefined, {
            onSuccess: () => toast.success("Syncing with HubSpot. This takes a minute or two."),
            onError: (err) =>
                errCode(err) === "crm_sync_running"
                    ? toast(errMessage(err, "A sync just started."))
                    : toast.error(errMessage(err, "Could not start a sync")),
        });
    }

    async function reconnect() {
        if (settings.connection_id && settings.account) {
            await oauth.reconnect(settings.connection_id);
            return;
        }
        // The connection is gone: connect again and point the CRM at the new one.
        const conn = await oauth.connect();
        if (!conn) return;
        try {
            await update.mutateAsync({ connection_id: conn.id });
        } catch (err) {
            toast.error(errMessage(err, "Could not use the new connection"));
        }
    }

    function stopUsingHubSpot() {
        confirm.show(
            "Stop using HubSpot as your CRM? Warmbly goes back to its own deals, tasks and notes. Records already mirrored stay in Warmbly, nothing is deleted in HubSpot, and activity stops being logged there.",
            async () => {
                try {
                    await update.mutateAsync({ provider: "native" });
                    toast.success("Back on Warmbly's own CRM");
                } catch (err) {
                    toast.error(errMessage(err, "Could not switch back"));
                }
            },
        );
    }

    const account = settings.account;
    const lost = !account;
    const disabled = !canManage;

    return (
        <div className="px-3 sm:px-5 py-4 sm:py-6">
            <div className="max-w-4xl mx-auto space-y-4">
                <div>
                    <Link
                        to="/app/integrations"
                        className="inline-flex items-center gap-1 h-6 -ml-1.5 px-1.5 mb-2 rounded-md text-[11.5px] text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
                    >
                        <ArrowLeftIcon className="w-3 h-3" />
                        Integrations
                    </Link>
                    <div className="flex flex-wrap items-start gap-3">
                        <span className="size-10 rounded-lg bg-orange-50 inline-flex items-center justify-center shrink-0">
                            <HubSpotMark className="w-5 h-5" />
                        </span>
                        <div className="min-w-0 flex-1 basis-48">
                            <div className="flex flex-wrap items-center gap-2">
                                <h1 className="text-[18px] font-semibold text-slate-900 tracking-tight">HubSpot</h1>
                                <StatusPill tone={lost || needsReconnect ? "amber" : account?.health === "degraded" ? "amber" : "emerald"}>
                                    {lost ? "Disconnected" : needsReconnect ? "Needs reconnect" : account?.health === "degraded" ? "Degraded" : "Your CRM"}
                                </StatusPill>
                                {draft && <SaveStatus status={autosave.status} onRetry={autosave.retry} />}
                            </div>
                            <p className="text-[12px] text-slate-500 truncate">
                                {account?.name || "HubSpot account"}
                                {account?.external_id && <span className="text-slate-400"> · Portal {account.external_id}</span>}
                            </p>
                            <HubSpotSyncedAt at={health.data?.last_synced_at} className="mt-0.5" />
                        </div>
                        <div className="flex flex-wrap items-center gap-1.5">
                            <button
                                type="button"
                                onClick={runSync}
                                disabled={syncNow.isPending || lost}
                                className="h-7 px-2.5 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] text-slate-700 hover:text-slate-900 inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                            >
                                <RefreshCwIcon className={cn("w-3 h-3", syncNow.isPending && "animate-spin")} />
                                Sync now
                            </button>
                            {appUrl && (
                                <a
                                    href={appUrl}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="h-7 px-2.5 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] text-slate-700 hover:text-orange-700 inline-flex items-center gap-1.5 transition-colors"
                                >
                                    <HubSpotMark className="w-3 h-3" />
                                    Open HubSpot
                                    <ExternalLinkIcon className="w-3 h-3 opacity-60" />
                                </a>
                            )}
                            {connection && (
                                <button
                                    type="button"
                                    onClick={() => setManageOpen(true)}
                                    className="h-7 px-2.5 rounded-md text-[12px] text-slate-600 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center gap-1.5 transition-colors"
                                >
                                    <PlugIcon className="w-3 h-3" />
                                    Connection
                                </button>
                            )}
                        </div>
                    </div>
                </div>

                {(lost || needsReconnect) && (
                    <div className="rounded-md border border-amber-200 bg-amber-50 px-3.5 py-3 flex flex-col sm:flex-row sm:items-center gap-2.5">
                        <AlertTriangleIcon className="hidden sm:block w-4 h-4 text-amber-600 shrink-0" />
                        <div className="min-w-0 flex-1">
                            <p className="text-[12.5px] font-medium text-amber-900">
                                {lost ? "The HubSpot connection was removed" : "Reconnect HubSpot to keep syncing"}
                            </p>
                            <p className="text-[11.5px] text-amber-800 leading-relaxed">
                                {lost
                                    ? "Nothing reaches HubSpot until it is connected again."
                                    : account?.missing_scopes?.length
                                      ? "HubSpot has not granted every permission CRM mode needs. Reconnect and approve the request."
                                      : "HubSpot stopped accepting Warmbly's access. Reconnect to pick up where it left off."}
                            </p>
                        </div>
                        <button
                            type="button"
                            onClick={() => void reconnect()}
                            disabled={oauth.busy || !canManage}
                            className="h-7 px-2.5 rounded-md bg-amber-500 hover:bg-amber-600 text-white text-[12px] font-medium inline-flex items-center gap-1.5 shrink-0 transition-colors disabled:opacity-60"
                        >
                            {oauth.busy ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <RefreshCwIcon className="w-3 h-3" />}
                            {lost ? "Connect HubSpot" : "Reconnect HubSpot"}
                        </button>
                    </div>
                )}

                {!canManage && (
                    <p className="text-[11.5px] text-slate-500">
                        You can see these settings. Members who manage workspace settings can change them.
                    </p>
                )}

                <SyncHealthCard canManage={canManage} />

                {!draft ? (
                    <p className="text-[12px] text-slate-400 inline-flex items-center gap-1.5">
                        <Loader2Icon className="w-3 h-3 animate-spin" />
                        Loading settings…
                    </p>
                ) : (
                    <>
                        {metadata.error && (
                            <p className="text-[11.5px] text-amber-700">{errMessage(metadata.error, "Could not read your HubSpot properties.")}</p>
                        )}
                        <Card title="Contacts and fields" description="How Warmbly contacts and HubSpot contacts stay in step." badge={<HubSpotBadge />}>
                            <ContactsEditor config={draft} patch={patch} disabled={disabled} />
                            <div className="space-y-2 pt-4 border-t border-slate-100">
                                <SubLabel>Field mapping</SubLabel>
                                <FieldMappingTable
                                    config={draft}
                                    patch={patch}
                                    properties={metadata.data?.properties ?? []}
                                    customKeys={customKeys.data ?? []}
                                    disabled={disabled}
                                />
                            </div>
                        </Card>

                        <Card
                            title="People"
                            description="HubSpot users matched to workspace members. Records owned by someone who is not a member show their HubSpot name."
                        >
                            <OwnersTable disabled={disabled} />
                        </Card>

                        <Card title="Activity" description="Which Warmbly events are logged on the HubSpot contact timeline.">
                            <ActivityEditor config={draft} patch={patch} disabled={disabled} />
                            <div className="pt-4 border-t border-slate-100">
                                <WarmblyPropertiesEditor config={draft} patch={patch} disabled={disabled} />
                            </div>
                        </Card>

                        <Card title="Rules" description="What a good reply does in HubSpot, and when HubSpot should stop a campaign.">
                            <div className="space-y-3">
                                <SubLabel>When someone replies with interest</SubLabel>
                                <ReplyOutcomeEditor config={draft} patch={patch} metadata={metadata.data} disabled={disabled} />
                                {issue && <p className="text-[11.5px] text-amber-700">{issue} Nothing is saved until then.</p>}
                            </div>
                            <div className="space-y-3 pt-4 border-t border-slate-100">
                                <SubLabel>Stop the campaign for a contact when</SubLabel>
                                <ExitRulesEditor config={draft} patch={patch} metadata={metadata.data} disabled={disabled} />
                            </div>
                            <div className="space-y-3 pt-4 border-t border-slate-100">
                                <SubLabel>When importing a HubSpot list, skip contacts who</SubLabel>
                                <GuardsEditor config={draft} patch={patch} metadata={metadata.data} disabled={disabled} />
                            </div>
                        </Card>

                        <div className="grid md:grid-cols-2 gap-4">
                            <Card
                                title="Pipelines to mirror"
                                description="The HubSpot deal pipelines that show up in Warmbly. Leave it on all pipelines unless some belong to another team."
                            >
                                <PipelinesEditor config={draft} patch={patch} metadata={metadata.data} disabled={disabled} />
                            </Card>
                            <Card
                                title="Contact properties shown in Warmbly"
                                description="Extra HubSpot properties shown on contacts and inbox threads, next to Owner, Lifecycle stage and Lead status."
                            >
                                <DisplayPropertiesEditor
                                    config={draft}
                                    patch={patch}
                                    properties={metadata.data?.properties ?? []}
                                    disabled={disabled}
                                />
                            </Card>
                        </div>
                    </>
                )}

                {canManage && <BackfillCard />}

                {canManage && (
                    <section className="rounded-md border border-rose-200 bg-white">
                        <div className="px-4 py-3 flex flex-col sm:flex-row sm:items-center gap-3">
                            <div className="min-w-0 flex-1">
                                <h3 className="text-[12.5px] font-semibold text-rose-700">Stop using HubSpot as your CRM</h3>
                                <p className="text-[11.5px] text-slate-500 mt-0.5 leading-relaxed">
                                    Warmbly goes back to its own deals, tasks and notes. Everything already mirrored stays in
                                    Warmbly, and nothing is deleted in HubSpot. HubSpot stays connected for automations.
                                </p>
                            </div>
                            <button
                                type="button"
                                onClick={stopUsingHubSpot}
                                disabled={update.isPending}
                                className="h-7 px-3 rounded-md border border-rose-200 text-[12px] font-medium text-rose-600 hover:bg-rose-50 hover:border-rose-300 inline-flex items-center gap-1.5 shrink-0 transition-colors disabled:opacity-60"
                            >
                                Switch back to Warmbly CRM
                            </button>
                        </div>
                    </section>
                )}
            </div>

            {manageOpen && connection && (
                <ConnectionDetail connection={connection} entry={entry} onClose={() => setManageOpen(false)} />
            )}
        </div>
    );
}

// Records created in Warmbly's own CRM (before the switch, or while it was
// off) can be copied into HubSpot at any time.
function BackfillCard() {
    const preview = useCrmBackfillPreview();
    const start = useStartCrmBackfill();
    const [copy, setCopy] = React.useState<CRMBackfillRequest>({ deals: true, tasks: true, notes: true });
    const p = preview.data;
    const items = (
        [
            ["deals", "deal"],
            ["tasks", "task"],
            ["notes", "note"],
        ] as const
    ).filter(([k]) => (p?.[k] ?? 0) > 0);
    if (items.length === 0) return null;
    const chosen = items.some(([k]) => copy[k]);

    function run() {
        start.mutate(
            { deals: copy.deals && (p?.deals ?? 0) > 0, tasks: copy.tasks && (p?.tasks ?? 0) > 0, notes: copy.notes && (p?.notes ?? 0) > 0 },
            {
                onSuccess: () => toast.success("Copying into HubSpot. Progress shows in Sync health."),
                onError: (err) => toast.error(errMessage(err, "Could not start copying")),
            },
        );
    }

    return (
        <Card
            title="Warmbly-only records"
            description={`${joinList(items.map(([k, one]) => plural(p?.[k] ?? 0, one)))} exist only in Warmbly. Copy them into HubSpot once; nothing is copied twice.`}
            actions={
                <button
                    type="button"
                    onClick={run}
                    disabled={!chosen || start.isPending}
                    title={chosen ? undefined : "Choose what to copy first"}
                    className="h-7 px-2.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                >
                    {start.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <CopyPlusIcon className="w-3 h-3" />}
                    Copy into HubSpot
                </button>
            }
        >
            <div className="flex flex-wrap gap-x-5 gap-y-1.5">
                {items.map(([k, one]) => (
                    <label key={k} className="flex items-center gap-2 text-[12.5px] text-slate-700 cursor-pointer">
                        <Checkbox tone="slate" checked={copy[k]} onChange={(e) => setCopy((c) => ({ ...c, [k]: e.target.checked }))} />
                        {plural(p?.[k] ?? 0, one)}
                    </label>
                ))}
            </div>
        </Card>
    );
}

function StatusPill({ tone, children }: { tone: "emerald" | "amber"; children: React.ReactNode }) {
    return (
        <span
            className={cn(
                "inline-flex items-center gap-1 h-5 px-2 rounded-md border text-[10px] uppercase tracking-[0.12em] font-medium",
                tone === "emerald" ? "bg-emerald-50 text-emerald-700 border-emerald-200" : "bg-amber-50 text-amber-700 border-amber-200",
            )}
        >
            <span className={cn("size-1.5 rounded-full", tone === "emerald" ? "bg-emerald-500" : "bg-amber-500")} />
            {children}
        </span>
    );
}
