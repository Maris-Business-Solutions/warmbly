// HubSpot setup: five short steps, each pre-filled with a sensible default so
// "Looks good" all the way through is a complete setup. The switch happens at
// the end of step one, because the remaining steps need HubSpot's own users,
// stages and properties; every step after it saves as you continue.

import React from "react";
import { Link } from "react-router-dom";
import { AnimatePresence, motion } from "framer-motion";
import { useQueryClient } from "@tanstack/react-query";
import {
    AlertCircleIcon,
    ArrowLeftIcon,
    CheckCircle2Icon,
    CheckIcon,
    ChevronLeftIcon,
    ChevronRightIcon,
    Loader2Icon,
    RefreshCwIcon,
    SparklesIcon,
} from "lucide-react";
import toast from "react-hot-toast";

import { HubSpotMark } from "@/components/app/crm/HubSpot";
import { Checkbox } from "@/components/ui/checkbox";
import { SelectMenu } from "@/components/ui/select-menu";
import { useConfirm } from "@/hooks/context/confirm";
import { usePermission } from "@/hooks/usePermission";
import updateCrmSettings from "@/lib/api/client/app/crm/provider/updateCrmSettings";
import useCustomFieldKeys from "@/lib/api/hooks/app/contacts/useCustomFieldKeys";
import { useCrmBackfillPreview, useStartCrmBackfill } from "@/lib/api/hooks/app/crm/provider/useCrmBackfill";
import useCrmMetadata from "@/lib/api/hooks/app/crm/provider/useCrmMetadata";
import useUpdateCrmSettings from "@/lib/api/hooks/app/crm/provider/useUpdateCrmSettings";
import type { CRMBackfillRequest, CRMProviderConfig, CRMSettings } from "@/lib/api/models/app/crm/CRMProvider";
import type { IntegrationConnection } from "@/lib/api/models/app/integrations/Integration";
import { cn } from "@/lib/utils";

import {
    ActivityEditor,
    ContactsEditor,
    ExitRulesEditor,
    FieldMappingTable,
    GuardsEditor,
    OwnersTable,
    ReplyOutcomeEditor,
    SubLabel,
    WarmblyPropertiesEditor,
} from "./editors";
import { useHubSpotOAuth, useLeaveGuard } from "./hooks";
import {
    type ConfigPatch,
    errCode,
    errMessage,
    joinList,
    normalizeConfig,
    plural,
    replyOutcomeIssue,
    sameConfig,
} from "./shared";

type StepKey = "switch" | "contacts" | "people" | "activity" | "rules";

const STEPS: { key: StepKey; label: string; title: string; description: string }[] = [
    {
        key: "switch",
        label: "HubSpot",
        title: "Use HubSpot as your CRM",
        description: "Warmbly stops keeping its own CRM records and works on HubSpot's instead.",
    },
    {
        key: "contacts",
        label: "Contacts",
        title: "Contacts and fields",
        description: "How Warmbly contacts and HubSpot contacts stay in step.",
    },
    {
        key: "people",
        label: "People",
        title: "Match your team",
        description: "HubSpot users are matched to workspace members by email, so owners and assignees line up on both sides.",
    },
    {
        key: "activity",
        label: "Activity",
        title: "What shows up in HubSpot",
        description: "Choose which Warmbly events are logged on the HubSpot contact timeline.",
    },
    {
        key: "rules",
        label: "Rules",
        title: "Rules",
        description: "What a good reply does in HubSpot, and when HubSpot should stop a campaign.",
    },
];

const CHANGES = [
    "Deals, tasks, notes and pipelines become HubSpot's. Changes made in Warmbly write straight through to HubSpot.",
    "Sends, replies, bounces and meetings are logged on the HubSpot timeline as real emails.",
    "Owner, Lifecycle stage and Lead status show on every contact and inbox thread.",
    "Campaigns stop for a contact when HubSpot says they moved on, like a new deal.",
    "Contacts can be imported straight from HubSpot lists.",
];

const paneVariants = {
    enter: (dir: 1 | -1) => ({ x: dir * 28, opacity: 0 }),
    center: { x: 0, opacity: 1 },
    exit: (dir: 1 | -1) => ({ x: dir * -28, opacity: 0 }),
};

export default function SetupWizard({
    settings,
    connections,
}: {
    settings: CRMSettings;
    connections: IntegrationConnection[];
}) {
    const queryClient = useQueryClient();
    const confirm = useConfirm();
    const canManage = usePermission("MANAGE_SETTINGS");
    const update = useUpdateCrmSettings();
    const startBackfill = useStartCrmBackfill();
    const oauth = useHubSpotOAuth();

    const switched = settings.provider === "hubspot";
    const [connectionId, setConnectionId] = React.useState(
        () => settings.connection_id ?? (connections.find((c) => c.status === "connected") ?? connections[0])?.id ?? "",
    );
    const connection = connections.find((c) => c.id === connectionId);

    const [step, setStep] = React.useState(switched ? 1 : 0);
    const [reached, setReached] = React.useState(switched ? 1 : 0);
    const [direction, setDirection] = React.useState<1 | -1>(1);
    const [busy, setBusy] = React.useState<null | "switch" | "save" | "finish" | "cancel">(null);
    const [nudged, setNudged] = React.useState(false);
    const [serverIssue, setServerIssue] = React.useState<string | null>(null);
    const [needsReauth, setNeedsReauth] = React.useState(false);

    const [draft, setDraft] = React.useState<CRMProviderConfig>(() => normalizeConfig(settings.config));
    const [saved, setSaved] = React.useState<CRMProviderConfig>(() => normalizeConfig(settings.config));
    const patch: ConfigPatch = React.useCallback((fn) => setDraft((c) => fn(c)), []);
    const dirty = switched && !sameConfig(draft, saved);

    const preview = useCrmBackfillPreview(canManage);
    const [copy, setCopy] = React.useState<CRMBackfillRequest>({ deals: true, tasks: true, notes: true });
    const metadata = useCrmMetadata(switched);
    const customKeys = useCustomFieldKeys();

    const allowLeave = useLeaveGuard(
        dirty,
        "Leave HubSpot setup? The changes on this step are not saved yet. HubSpot stays your CRM with what was saved so far.",
    );

    const current = STEPS[step];
    const last = STEPS.length - 1;

    function issueFor(i: number): string | null {
        const key = STEPS[i].key;
        if (!canManage) return "Only members who can manage workspace settings can set up HubSpot.";
        if (key === "switch" && !connectionId) return "Connect a HubSpot account first.";
        if (key === "rules") return replyOutcomeIssue(draft);
        return null;
    }
    const issue = serverIssue ?? issueFor(step);

    React.useEffect(() => {
        setNudged(false);
        setServerIssue(null);
    }, [step]);
    React.useEffect(() => setServerIssue(null), [draft]);

    function goTo(i: number) {
        if (i === step || busy) return;
        if (i > step) {
            for (let s = step; s < i; s++) {
                if (issueFor(s)) {
                    setNudged(true);
                    return;
                }
            }
            if (i > reached) return;
        }
        setDirection(i > step ? 1 : -1);
        setStep(i);
    }

    function advance() {
        setDirection(1);
        setStep((s) => s + 1);
        setReached((r) => Math.max(r, step + 1));
    }

    async function saveDraft(): Promise<boolean> {
        if (!dirty) return true;
        setBusy("save");
        try {
            const res = await updateCrmSettings({ config: draft });
            queryClient.setQueryData(["crm", "settings"], res);
            setSaved(draft);
            return true;
        } catch (err) {
            setServerIssue(errMessage(err, "Could not save these settings"));
            setNudged(true);
            return false;
        } finally {
            setBusy(null);
        }
    }

    async function doSwitch() {
        setBusy("switch");
        try {
            await update.mutateAsync({ provider: "hubspot", connection_id: connectionId });
            setNeedsReauth(false);
            advance();
        } catch (err) {
            if (errCode(err) === "crm_reauth_required") {
                setNeedsReauth(true);
            } else {
                setServerIssue(errMessage(err, "Could not switch to HubSpot"));
                setNudged(true);
            }
        } finally {
            setBusy(null);
        }
    }

    async function finish() {
        setBusy("finish");
        try {
            await update.mutateAsync({ provider: "hubspot", connection_id: connectionId, config: draft, complete_setup: true });
            allowLeave();
            setSaved(draft);
            const counts = preview.data;
            const want: CRMBackfillRequest = {
                deals: copy.deals && (counts?.deals ?? 0) > 0,
                tasks: copy.tasks && (counts?.tasks ?? 0) > 0,
                notes: copy.notes && (counts?.notes ?? 0) > 0,
            };
            if (want.deals || want.tasks || want.notes) {
                try {
                    await startBackfill.mutateAsync(want);
                    toast.success("HubSpot is your CRM now. Copying your Warmbly records into HubSpot.");
                } catch (err) {
                    toast.error(`Setup is done, but copying records did not start: ${errMessage(err, "try again from HubSpot settings")}`);
                }
            } else {
                toast.success("HubSpot is your CRM now");
            }
        } catch (err) {
            setServerIssue(errMessage(err, "Could not finish setup"));
            setNudged(true);
        } finally {
            setBusy(null);
        }
    }

    async function next() {
        if (issue) {
            setNudged(true);
            return;
        }
        if (current.key === "switch") {
            if (!switched) return doSwitch();
            return advance();
        }
        if (step === last) return finish();
        if (await saveDraft()) advance();
    }

    function cancelSetup() {
        confirm.show(
            "Stop setting up HubSpot? Your workspace goes back to Warmbly's own CRM. Nothing is deleted in Warmbly or in HubSpot.",
            async () => {
                setBusy("cancel");
                try {
                    allowLeave();
                    await update.mutateAsync({ provider: "native" });
                    setDraft(saved);
                    setDirection(-1);
                    setStep(0);
                    setReached(0);
                    toast.success("Back on Warmbly's own CRM");
                } catch (err) {
                    toast.error(errMessage(err, "Could not switch back"));
                } finally {
                    setBusy(null);
                }
            },
        );
    }

    async function reconnect() {
        if (!connectionId) return;
        if (await oauth.reconnect(connectionId)) setNeedsReauth(false);
    }

    const primaryLabel =
        current.key === "switch"
            ? switched
                ? "Continue"
                : "Switch to HubSpot"
            : step === last
              ? "Finish setup"
              : dirty
                ? "Save and continue"
                : "Looks good";

    return (
        <div className="px-3 sm:px-5 py-4 sm:py-6">
            <div className="max-w-3xl mx-auto">
                <Link
                    to="/app/integrations"
                    className="inline-flex items-center gap-1 h-6 -ml-1.5 px-1.5 mb-2 rounded-md text-[11.5px] text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
                >
                    <ArrowLeftIcon className="w-3 h-3" />
                    Integrations
                </Link>
                <div className="flex flex-wrap items-center gap-3 mb-4">
                    <span className="size-9 rounded-lg bg-orange-50 inline-flex items-center justify-center shrink-0">
                        <HubSpotMark className="w-5 h-5" />
                    </span>
                    <div className="min-w-0 flex-1">
                        <h1 className="text-[18px] font-semibold text-slate-900 tracking-tight">Set up HubSpot</h1>
                        <p className="text-[12px] text-slate-500 truncate">
                            {connection?.external_account_name || settings.account?.name || "HubSpot"}
                            {" · "}About two minutes. Everything stays editable afterwards.
                        </p>
                    </div>
                    {switched && (
                        <button
                            type="button"
                            onClick={cancelSetup}
                            disabled={!!busy || !canManage}
                            className="h-7 px-2.5 rounded-md text-[12px] text-slate-500 hover:text-rose-600 hover:bg-rose-50 inline-flex items-center gap-1.5 transition-colors disabled:opacity-50"
                        >
                            {busy === "cancel" && <Loader2Icon className="w-3 h-3 animate-spin" />}
                            Cancel setup
                        </button>
                    )}
                </div>

                <div className="rounded-md border border-slate-200 bg-white overflow-hidden">
                    <Stepper step={step} reached={reached} goTo={goTo} />

                    <div className="relative overflow-hidden">
                        <AnimatePresence mode="wait" initial={false} custom={direction}>
                            <motion.div
                                key={current.key}
                                custom={direction}
                                variants={paneVariants}
                                initial="enter"
                                animate="center"
                                exit="exit"
                                transition={{ duration: 0.18, ease: [0.22, 1, 0.36, 1] }}
                                className="px-4 sm:px-6 py-5"
                            >
                                <h2 className="text-[15px] font-semibold text-slate-900">{current.title}</h2>
                                <p className="text-[12px] text-slate-500 mt-0.5 mb-5 leading-relaxed">{current.description}</p>

                                {current.key === "switch" && (
                                    <SwitchStep
                                        switched={switched}
                                        connections={connections}
                                        connectionId={connectionId}
                                        setConnectionId={setConnectionId}
                                        needsReauth={needsReauth || connection?.status === "reauth_required"}
                                        reconnect={reconnect}
                                        reconnecting={oauth.busy}
                                        preview={preview.data}
                                        copy={copy}
                                        setCopy={setCopy}
                                    />
                                )}
                                {current.key === "contacts" && (
                                    <div className="space-y-6">
                                        <ContactsEditor config={draft} patch={patch} disabled={!canManage} />
                                        <div className="space-y-2">
                                            <SubLabel>Field mapping</SubLabel>
                                            <MetadataNote loading={metadata.isLoading} error={metadata.error} />
                                            <FieldMappingTable
                                                config={draft}
                                                patch={patch}
                                                properties={metadata.data?.properties ?? []}
                                                customKeys={customKeys.data ?? []}
                                                disabled={!canManage}
                                            />
                                        </div>
                                    </div>
                                )}
                                {current.key === "people" && <OwnersTable waiting disabled={!canManage} />}
                                {current.key === "activity" && (
                                    <div className="space-y-5">
                                        <ActivityEditor config={draft} patch={patch} disabled={!canManage} />
                                        <div className="pt-4 border-t border-slate-100">
                                            <WarmblyPropertiesEditor config={draft} patch={patch} disabled={!canManage} />
                                        </div>
                                    </div>
                                )}
                                {current.key === "rules" && (
                                    <div className="space-y-6">
                                        <MetadataNote loading={metadata.isLoading} error={metadata.error} />
                                        <div className="space-y-3">
                                            <SubLabel>When someone replies with interest</SubLabel>
                                            <ReplyOutcomeEditor config={draft} patch={patch} metadata={metadata.data} disabled={!canManage} />
                                        </div>
                                        <div className="space-y-3 pt-5 border-t border-slate-100">
                                            <SubLabel>Stop the campaign for a contact when</SubLabel>
                                            <ExitRulesEditor config={draft} patch={patch} metadata={metadata.data} disabled={!canManage} />
                                        </div>
                                        <div className="space-y-3 pt-5 border-t border-slate-100">
                                            <SubLabel>When importing a HubSpot list, skip contacts who</SubLabel>
                                            <GuardsEditor config={draft} patch={patch} metadata={metadata.data} disabled={!canManage} />
                                        </div>
                                    </div>
                                )}
                            </motion.div>
                        </AnimatePresence>
                    </div>

                    <div className="px-3 min-h-12 py-1.5 sm:py-0 sm:h-12 border-t border-slate-200 flex items-center gap-1.5 bg-slate-50/30">
                        {step > 0 ? (
                            <button
                                type="button"
                                onClick={() => goTo(step - 1)}
                                disabled={!!busy}
                                className="h-7 px-2.5 rounded-md text-[12px] text-slate-700 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center gap-1 transition-colors disabled:opacity-50"
                            >
                                <ChevronLeftIcon className="w-3 h-3" />
                                Back
                            </button>
                        ) : (
                            <span className="text-[11px] text-slate-400 pl-1 hidden sm:inline">
                                {switched ? "HubSpot is your CRM. Finish the remaining steps when you are ready." : "You can switch back at any time."}
                            </span>
                        )}
                        <div className="ml-auto flex items-center gap-2 min-w-0">
                            <AnimatePresence initial={false}>
                                {nudged && issue && (
                                    <motion.span
                                        key={issue}
                                        initial={{ opacity: 0, x: 6 }}
                                        animate={{ opacity: 1, x: 0 }}
                                        exit={{ opacity: 0, x: 6 }}
                                        transition={{ duration: 0.14 }}
                                        role="status"
                                        className="text-[11.5px] text-amber-700 inline-flex items-center gap-1 min-w-0"
                                    >
                                        <AlertCircleIcon className="w-3 h-3 shrink-0" />
                                        <span className="truncate" title={issue}>{issue}</span>
                                    </motion.span>
                                )}
                            </AnimatePresence>
                            <button
                                type="button"
                                onClick={() => void next()}
                                disabled={!!busy}
                                className="h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors shrink-0 disabled:opacity-60"
                            >
                                {busy && busy !== "cancel" ? (
                                    <Loader2Icon className="w-3 h-3 animate-spin" />
                                ) : step === last ? (
                                    <CheckIcon className="w-3 h-3" />
                                ) : null}
                                {busy === "switch" ? "Switching…" : busy === "finish" ? "Finishing…" : primaryLabel}
                                {!busy && step < last && <ChevronRightIcon className="w-3 h-3" />}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

function MetadataNote({ loading, error }: { loading: boolean; error: unknown }) {
    if (loading) {
        return (
            <p className="text-[11.5px] text-slate-400 inline-flex items-center gap-1.5">
                <Loader2Icon className="w-3 h-3 animate-spin" />
                Loading your HubSpot properties and stages…
            </p>
        );
    }
    if (error) {
        return <p className="text-[11.5px] text-amber-700">{errMessage(error, "Could not read your HubSpot properties.")}</p>;
    }
    return null;
}

function SwitchStep({
    switched,
    connections,
    connectionId,
    setConnectionId,
    needsReauth,
    reconnect,
    reconnecting,
    preview,
    copy,
    setCopy,
}: {
    switched: boolean;
    connections: IntegrationConnection[];
    connectionId: string;
    setConnectionId: (id: string) => void;
    needsReauth: boolean;
    reconnect: () => void;
    reconnecting: boolean;
    preview?: { deals: number; tasks: number; notes: number };
    copy: CRMBackfillRequest;
    setCopy: React.Dispatch<React.SetStateAction<CRMBackfillRequest>>;
}) {
    const items = (
        [
            ["deals", "deal"],
            ["tasks", "task"],
            ["notes", "note"],
        ] as const
    ).filter(([k]) => (preview?.[k] ?? 0) > 0);

    return (
        <div className="space-y-5">
            {switched ? (
                <div className="rounded-md border border-emerald-200 bg-emerald-50/60 px-3 py-2.5 flex items-start gap-2">
                    <CheckCircle2Icon className="w-3.5 h-3.5 text-emerald-600 mt-0.5 shrink-0" />
                    <p className="text-[12px] text-emerald-800 leading-relaxed">
                        HubSpot is your CRM. Warmbly is pulling your owners, pipelines, deals and tasks now.
                    </p>
                </div>
            ) : (
                <ul className="space-y-2">
                    {CHANGES.map((c) => (
                        <li key={c} className="flex items-start gap-2 text-[12.5px] text-slate-700 leading-relaxed">
                            <CheckCircle2Icon className="w-3.5 h-3.5 text-emerald-500 mt-0.5 shrink-0" />
                            <span>{c}</span>
                        </li>
                    ))}
                </ul>
            )}

            {!switched && connections.length > 1 && (
                <div className="max-w-sm">
                    <SubLabel className="mb-1.5">HubSpot account</SubLabel>
                    <SelectMenu
                        value={connectionId}
                        onChange={setConnectionId}
                        options={connections.map((c) => ({ value: c.id, label: c.external_account_name || c.label }))}
                        fullWidth
                        aria-label="HubSpot account"
                    />
                </div>
            )}

            {needsReauth && !switched && (
                <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 flex flex-col sm:flex-row sm:items-center gap-2">
                    <p className="text-[12px] text-amber-800 leading-relaxed flex-1">
                        HubSpot needs a few more permissions for CRM mode (deals, companies and owners). Reconnect to grant them, then switch.
                    </p>
                    <button
                        type="button"
                        onClick={reconnect}
                        disabled={reconnecting}
                        className="h-7 px-2.5 rounded-md bg-amber-500 hover:bg-amber-600 text-white text-[12px] font-medium inline-flex items-center gap-1.5 shrink-0 transition-colors disabled:opacity-60"
                    >
                        {reconnecting ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <RefreshCwIcon className="w-3 h-3" />}
                        Reconnect HubSpot
                    </button>
                </div>
            )}

            {items.length > 0 && (
                <div className="rounded-md border border-slate-200 bg-slate-50/50 px-3.5 py-3 space-y-2.5">
                    <div className="flex items-start gap-2">
                        <SparklesIcon className="w-3.5 h-3.5 text-sky-600 mt-0.5 shrink-0" />
                        <div>
                            <p className="text-[12.5px] font-medium text-slate-900">Bring your Warmbly CRM along</p>
                            <p className="text-[11.5px] text-slate-500 leading-relaxed">
                                You have {joinList(items.map(([k, one]) => plural(preview?.[k] ?? 0, one)))} in Warmbly's own CRM.
                                Copy them into HubSpot once, so nothing is left behind.
                            </p>
                        </div>
                    </div>
                    <div className="pl-5.5 space-y-1.5">
                        {items.map(([k, one]) => (
                            <label key={k} className="flex items-center gap-2 text-[12.5px] text-slate-700 cursor-pointer w-fit">
                                <Checkbox
                                    tone="slate"
                                    checked={copy[k]}
                                    onChange={(e) => setCopy((c) => ({ ...c, [k]: e.target.checked }))}
                                />
                                Copy {plural(preview?.[k] ?? 0, one)}
                            </label>
                        ))}
                    </div>
                    <p className="pl-5.5 text-[10.5px] text-slate-400">
                        Copying starts when you finish setup. The records stay in Warmbly either way.
                    </p>
                </div>
            )}

            {!switched && (
                <p className={cn("text-[11.5px] text-slate-500 leading-relaxed")}>
                    The switch applies to everyone in this workspace as soon as you press Switch to HubSpot. The next steps
                    are already filled in, so you can press Looks good through them, and change anything later.
                </p>
            )}
        </div>
    );
}

function Stepper({ step, reached, goTo }: { step: number; reached: number; goTo: (s: number) => void }) {
    return (
        <div className="px-4 sm:px-5 h-11 border-b border-slate-100 flex items-center bg-slate-50/40">
            {STEPS.map((s, i) => {
                const active = i === step;
                const done = i !== step && i < reached;
                const reachable = i <= reached;
                return (
                    <React.Fragment key={s.key}>
                        <button
                            type="button"
                            onClick={() => goTo(i)}
                            disabled={!reachable}
                            aria-current={active ? "step" : undefined}
                            className={cn(
                                "group inline-flex items-center gap-2 h-7 pl-1 pr-2 rounded-md shrink-0 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sky-100",
                                reachable && !active ? "hover:bg-slate-100" : "",
                                !reachable ? "cursor-default" : "",
                            )}
                        >
                            <span
                                className={cn(
                                    "relative size-5 rounded-full inline-flex items-center justify-center text-[10.5px] font-semibold tabular-nums transition-colors",
                                    done
                                        ? "bg-sky-600 text-white"
                                        : active
                                          ? "bg-white text-sky-700 ring-1 ring-inset ring-sky-600"
                                          : "bg-white text-slate-400 ring-1 ring-inset ring-slate-200",
                                )}
                            >
                                <AnimatePresence mode="wait" initial={false}>
                                    {done ? (
                                        <motion.span
                                            key="check"
                                            initial={{ scale: 0.4, opacity: 0 }}
                                            animate={{ scale: 1, opacity: 1 }}
                                            exit={{ scale: 0.4, opacity: 0 }}
                                            transition={{ duration: 0.16 }}
                                            className="inline-flex"
                                        >
                                            <CheckIcon className="w-3 h-3" strokeWidth={3} />
                                        </motion.span>
                                    ) : (
                                        <motion.span
                                            key="num"
                                            initial={{ scale: 0.4, opacity: 0 }}
                                            animate={{ scale: 1, opacity: 1 }}
                                            exit={{ scale: 0.4, opacity: 0 }}
                                            transition={{ duration: 0.16 }}
                                        >
                                            {i + 1}
                                        </motion.span>
                                    )}
                                </AnimatePresence>
                            </span>
                            <span
                                className={cn(
                                    "text-[11.5px] font-medium whitespace-nowrap",
                                    active ? "text-slate-900" : done ? "text-slate-600" : "text-slate-400",
                                    active ? "inline" : "hidden sm:inline",
                                )}
                            >
                                {s.label}
                            </span>
                        </button>
                        {i < STEPS.length - 1 && (
                            <span className="relative flex-1 h-px mx-1 sm:mx-2 bg-slate-200 min-w-3 overflow-hidden">
                                <motion.span
                                    initial={false}
                                    animate={{ scaleX: i < step ? 1 : 0 }}
                                    transition={{ duration: 0.25, ease: [0.22, 1, 0.36, 1] }}
                                    style={{ originX: 0 }}
                                    className="absolute inset-0 bg-sky-600"
                                />
                            </span>
                        )}
                    </React.Fragment>
                );
            })}
        </div>
    );
}
