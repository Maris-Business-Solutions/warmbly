// The connected CRM's settings (HubSpot or Pipedrive), one editor per concern.
// The setup wizard shows them one step at a time; the settings page as cards.

import React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeftRightIcon, ArrowRightIcon, ArrowLeftIcon, Loader2Icon, MailIcon, PlusIcon, XIcon } from "lucide-react";
import toast from "react-hot-toast";

import { SettingRow, Toggle } from "@/components/app/campaigns/preferences/components/CampaignPreferenceBoolBox";
import { SelectMenu, type SelectOption } from "@/components/ui/select-menu";
import { CrmMark } from "@/components/app/crm/crmProviders";
import listCrmOwners from "@/lib/api/client/app/crm/provider/listCrmOwners";
import useCrmSyncHealth from "@/lib/api/hooks/app/crm/provider/useCrmSyncHealth";
import useMapCrmOwner from "@/lib/api/hooks/app/crm/provider/useMapCrmOwner";
import useMembers from "@/lib/api/hooks/app/organizations/useMembers";
import type {
    CRMActivityLog,
    CRMFieldDirection,
    CRMMetadata,
    CRMOwner,
    CRMProperty,
    CRMProviderConfig,
} from "@/lib/api/models/app/crm/CRMProvider";
import type Pipeline from "@/lib/api/models/app/crm/Pipeline";
import { cn } from "@/lib/utils";

import { usePageCrm } from "./context";
import { useProviderPipelines } from "./hooks";
import { MultiPicker, SearchSelect } from "./pickers";
import {
    type ConfigPatch,
    MAPPABLE_FIELDS,
    directionOptions,
    errMessage,
    humanize,
    warmblyFieldLabel,
    withStoredValues,
} from "./shared";

// --- layout ------------------------------------------------------------------

export function Card({
    title,
    description,
    badge,
    actions,
    children,
    className,
}: {
    title: string;
    description?: React.ReactNode;
    badge?: React.ReactNode;
    actions?: React.ReactNode;
    children: React.ReactNode;
    className?: string;
}) {
    return (
        <section className={cn("rounded-md border border-slate-200 bg-white", className)}>
            <header className="px-4 py-3 border-b border-slate-200 flex flex-wrap items-start gap-x-3 gap-y-2">
                <div className="min-w-0 flex-1 basis-56">
                    <h3 className="text-[12.5px] font-semibold text-slate-900 flex items-center gap-1.5">
                        {title}
                        {badge}
                    </h3>
                    {description && <p className="text-[11.5px] text-slate-500 mt-0.5 leading-relaxed">{description}</p>}
                </div>
                {actions && <div className="flex items-center gap-1.5 shrink-0">{actions}</div>}
            </header>
            <div className="px-4 py-4 space-y-4">{children}</div>
        </section>
    );
}

export function SubLabel({ children, className }: { children: React.ReactNode; className?: string }) {
    return <div className={cn("text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium", className)}>{children}</div>;
}

function ToggleSetting({
    title,
    description,
    value,
    onChange,
    disabled,
}: {
    title: string;
    description?: React.ReactNode;
    value: boolean;
    onChange: (v: boolean) => void;
    disabled?: boolean;
}) {
    return (
        <SettingRow
            title={title}
            description={description}
            control={<Toggle value={value} onChange={onChange} disabled={disabled} ariaLabel={title} />}
        />
    );
}

// --- contacts ------------------------------------------------------------------

export function ContactsEditor({ config, patch, disabled }: { config: CRMProviderConfig; patch: ConfigPatch; disabled?: boolean }) {
    const crm = usePageCrm();
    const pd = crm.id === "pipedrive";
    return (
        <div className="space-y-3.5">
            <ToggleSetting
                title={`Create ${crm.words.contacts} in ${crm.name}`}
                description={`When Warmbly emails someone ${crm.name} does not know yet, a ${crm.name} ${crm.words.contact} is created so the activity has somewhere to go.`}
                value={config.create_contacts}
                onChange={(v) => patch((c) => ({ ...c, create_contacts: v }))}
                disabled={disabled}
            />
            <ToggleSetting
                title={pd ? "Create organizations" : "Create companies"}
                description={
                    pd
                        ? "New people are linked to the Pipedrive organization named after their company (or their email domain), created if it does not exist."
                        : "New contacts are associated with a HubSpot company from their email domain, created if it does not exist."
                }
                value={config.create_companies}
                onChange={(v) => patch((c) => ({ ...c, create_companies: v }))}
                disabled={disabled}
            />
        </div>
    );
}

// --- field mapping -------------------------------------------------------------

const directionIcon = (d: CRMFieldDirection) =>
    d === "push" ? ArrowRightIcon : d === "pull" ? ArrowLeftIcon : ArrowLeftRightIcon;

export function FieldMappingTable({
    config,
    patch,
    properties,
    customKeys,
    disabled,
}: {
    config: CRMProviderConfig;
    patch: ConfigPatch;
    properties: CRMProperty[];
    customKeys: string[];
    disabled?: boolean;
}) {
    const crm = usePageCrm();
    const directions = directionOptions(crm.name);
    const directionSelect: SelectOption[] = directions.map((o) => ({ value: o.value, label: o.label }));
    const propNoun = `${crm.name} ${crm.words.property}`;
    const propByName = React.useMemo(() => new Map(properties.map((p) => [p.name, p])), [properties]);
    const propOptions = React.useMemo(
        () => properties.map((p) => ({ value: p.name, label: p.label, hint: p.read_only ? `${p.name} · read-only` : p.name })),
        [properties],
    );
    const rows = Object.entries(config.field_map).filter(([k]) => k !== "email");
    const available: SelectOption[] = [
        ...MAPPABLE_FIELDS.filter((f) => !(f in config.field_map)).map((f) => ({ value: f, label: warmblyFieldLabel(f), group: "Contact" })),
        ...customKeys
            .filter((k) => !(`custom:${k}` in config.field_map))
            .map((k) => ({ value: `custom:${k}`, label: k, group: "Custom fields" })),
    ];

    const [newField, setNewField] = React.useState("");
    const [newProp, setNewProp] = React.useState("");

    function setProperty(field: string, prop: string) {
        const readOnly = propByName.get(prop)?.read_only;
        patch((c) => ({
            ...c,
            field_map: { ...c.field_map, [field]: prop },
            field_direction: readOnly ? { ...c.field_direction, [field]: "pull" } : c.field_direction,
        }));
    }

    function setDirection(field: string, dir: CRMFieldDirection) {
        patch((c) => {
            const next = { ...c.field_direction };
            if (dir === "both") delete next[field];
            else next[field] = dir;
            return { ...c, field_direction: next };
        });
    }

    function remove(field: string) {
        patch((c) => {
            const map = { ...c.field_map };
            const dir = { ...c.field_direction };
            delete map[field];
            delete dir[field];
            return { ...c, field_map: map, field_direction: dir };
        });
    }

    function pickNewField(field: string) {
        setNewField(field);
        // Suggest the CRM property with the same name, when there is one.
        if (!newProp && field.startsWith("custom:")) {
            const key = field.slice("custom:".length).toLowerCase().replace(/[^a-z0-9]/g, "");
            const match = properties.find((p) => p.name.replace(/_/g, "") === key || p.label.toLowerCase().replace(/[^a-z0-9]/g, "") === key);
            if (match) setNewProp(match.name);
        }
    }

    function add() {
        if (!newField || !newProp) return;
        setProperty(newField, newProp);
        setNewField("");
        setNewProp("");
    }

    const gridCls = "sm:grid sm:grid-cols-[minmax(0,1fr)_20px_minmax(0,1.25fr)_132px_28px] sm:items-center sm:gap-2";

    return (
        <div className="space-y-2">
            <div className={cn("hidden px-1", gridCls)}>
                <SubLabel>Warmbly</SubLabel>
                <span />
                <SubLabel className="flex items-center gap-1">
                    <CrmMark provider={crm.id} className="w-3 h-3" />
                    {propNoun}
                </SubLabel>
                <SubLabel>When both change</SubLabel>
                <span />
            </div>

            <div className="rounded-md border border-slate-200 divide-y divide-slate-100">
                <div className={cn("px-3 py-2 flex flex-wrap items-center gap-2", gridCls)}>
                    <span className="text-[12.5px] text-slate-900 inline-flex items-center gap-1.5">
                        <MailIcon className="w-3 h-3 text-slate-400" />
                        Email
                    </span>
                    <ArrowLeftRightIcon className="w-3 h-3 text-slate-300 hidden sm:block" />
                    <span className="text-[12px] text-slate-600">Email</span>
                    <span className="text-[11px] text-slate-400 basis-full sm:basis-auto sm:col-span-2">
                        {crm.id === "pipedrive" ? "People" : "Contacts"} are matched by email
                    </span>
                </div>
                {rows.map(([field, prop]) => {
                    const dir = config.field_direction[field] ?? "both";
                    const Icon = directionIcon(dir);
                    const meta = propByName.get(prop);
                    const pushesReadOnly = meta?.read_only && dir !== "pull";
                    return (
                        <div key={field} className="px-3 py-2">
                            <div className={cn("flex flex-col gap-1.5", gridCls)}>
                                <div className="flex items-center gap-2 min-w-0">
                                    <span className="text-[12.5px] text-slate-900 truncate">{warmblyFieldLabel(field)}</span>
                                    {field.startsWith("custom:") && (
                                        <span className="text-[10px] text-slate-400 shrink-0">custom</span>
                                    )}
                                    <button
                                        type="button"
                                        onClick={() => remove(field)}
                                        disabled={disabled}
                                        aria-label={`Stop syncing ${warmblyFieldLabel(field)}`}
                                        className="sm:hidden ml-auto size-6 rounded-md text-slate-400 hover:text-rose-600 hover:bg-rose-50 inline-flex items-center justify-center disabled:opacity-40"
                                    >
                                        <XIcon className="w-3 h-3" />
                                    </button>
                                </div>
                                <Icon className="w-3 h-3 text-slate-400 hidden sm:block" />
                                <SearchSelect
                                    options={propOptions}
                                    value={prop}
                                    onChange={(v) => setProperty(field, v)}
                                    placeholder={`Choose a ${crm.words.property}`}
                                    searchPlaceholder={`Search ${crm.name} ${crm.words.properties}…`}
                                    ariaLabel={`${propNoun} for ${warmblyFieldLabel(field)}`}
                                    disabled={disabled}
                                />
                                <SelectMenu
                                    value={dir}
                                    onChange={(v) => setDirection(field, v as CRMFieldDirection)}
                                    options={directionSelect}
                                    fullWidth
                                    disabled={disabled}
                                    aria-label={`Which side wins for ${warmblyFieldLabel(field)}`}
                                />
                                <button
                                    type="button"
                                    onClick={() => remove(field)}
                                    disabled={disabled}
                                    aria-label={`Stop syncing ${warmblyFieldLabel(field)}`}
                                    className="hidden sm:inline-flex size-7 rounded-md text-slate-400 hover:text-rose-600 hover:bg-rose-50 items-center justify-center transition-colors disabled:opacity-40"
                                >
                                    <XIcon className="w-3.5 h-3.5" />
                                </button>
                            </div>
                            {pushesReadOnly && (
                                <p className="mt-1.5 text-[11px] text-amber-700">
                                    {meta?.label} is read-only in {crm.name}, so Warmbly can only read it. Choose {crm.name} wins.
                                </p>
                            )}
                        </div>
                    );
                })}
            </div>

            {!disabled && (
                <div className="rounded-md border border-dashed border-slate-200 px-3 py-2.5">
                    <div className={cn("flex flex-col gap-1.5", "sm:grid sm:grid-cols-[minmax(0,1fr)_20px_minmax(0,1.25fr)_auto] sm:items-center sm:gap-2")}>
                        <SelectMenu
                            value={newField}
                            onChange={pickNewField}
                            options={available}
                            placeholder={available.length ? "Add a Warmbly field" : "Every field is mapped"}
                            disabled={available.length === 0}
                            fullWidth
                            aria-label="Warmbly field to sync"
                        />
                        <ArrowLeftRightIcon className="w-3 h-3 text-slate-300 hidden sm:block" />
                        <SearchSelect
                            options={propOptions}
                            value={newProp}
                            onChange={setNewProp}
                            placeholder={propNoun}
                            searchPlaceholder={`Search ${crm.name} ${crm.words.properties}…`}
                            ariaLabel={`${propNoun} to sync with`}
                        />
                        <button
                            type="button"
                            onClick={add}
                            disabled={!newField || !newProp}
                            title={!newField ? "Choose a Warmbly field first" : !newProp ? `Choose the ${propNoun} it syncs with` : undefined}
                            className="h-7 px-2.5 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-slate-300 hover:text-slate-900 inline-flex items-center justify-center gap-1 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            <PlusIcon className="w-3 h-3" />
                            Add
                        </button>
                    </div>
                </div>
            )}

            <ul className="pt-1 space-y-0.5">
                {directions.map((o) => (
                    <li key={o.value} className="text-[11px] text-slate-500 leading-relaxed">
                        <span className="font-medium text-slate-700">{o.label}:</span> {o.hint}
                    </li>
                ))}
            </ul>
        </div>
    );
}

// --- activity ------------------------------------------------------------------

function activityRows(pipedrive: boolean): { key: keyof CRMActivityLog; title: string; description: string }[] {
    return pipedrive ? PIPEDRIVE_ACTIVITY_ROWS : ACTIVITY_ROWS;
}

const PIPEDRIVE_ACTIVITY_ROWS: { key: keyof CRMActivityLog; title: string; description: string }[] = [
    {
        key: "sent",
        title: "Emails sent",
        description: "Each campaign email is logged as a done Email activity on the person, their organization and open deal, with subject and body.",
    },
    { key: "replies", title: "Replies", description: "Replies are logged as done Email activities, so the conversation reads in order." },
    { key: "bounces", title: "Bounces", description: "The logged email is marked as bounced so the record shows the address no longer works." },
    { key: "unsubscribes", title: "Unsubscribes", description: "A note is added to the person when they unsubscribe from your campaigns." },
    { key: "meetings", title: "Meetings booked", description: "Calendly and Cal.com bookings become Meeting activities on the person and their open deal." },
    {
        key: "opens",
        title: "Opens",
        description: "Off by default. Apple Mail Privacy Protection opens almost every email on its own, so opens mostly add noise.",
    },
    { key: "clicks", title: "Clicks", description: "Off by default. Security scanners in company inboxes click links before people do." },
];

const ACTIVITY_ROWS: { key: keyof CRMActivityLog; title: string; description: string }[] = [
    { key: "sent", title: "Emails sent", description: "Each campaign email is logged on the contact's timeline as a real email, with subject and body." },
    { key: "replies", title: "Replies", description: "Replies land on the timeline as incoming emails, threaded with what you sent." },
    { key: "bounces", title: "Bounces", description: "A bounce is logged so the record shows the address no longer works." },
    { key: "unsubscribes", title: "Unsubscribes", description: "Logged when someone unsubscribes from your campaigns." },
    { key: "meetings", title: "Meetings booked", description: "Calendly and Cal.com bookings are logged as meetings on the contact." },
    {
        key: "opens",
        title: "Opens",
        description: "Off by default. Apple Mail Privacy Protection opens almost every email on its own, so opens mostly add noise to the timeline.",
    },
    {
        key: "clicks",
        title: "Clicks",
        description: "Off by default. Security scanners in company inboxes click links before people do.",
    },
];

export function ActivityEditor({ config, patch, disabled }: { config: CRMProviderConfig; patch: ConfigPatch; disabled?: boolean }) {
    const crm = usePageCrm();
    return (
        <div className="space-y-3.5">
            {activityRows(crm.id === "pipedrive").map((r) => (
                <ToggleSetting
                    key={r.key}
                    title={r.title}
                    description={r.description}
                    value={config.activity[r.key]}
                    onChange={(v) => patch((c) => ({ ...c, activity: { ...c.activity, [r.key]: v } }))}
                    disabled={disabled}
                />
            ))}
        </div>
    );
}

export function WarmblyPropertiesEditor({ config, patch, disabled }: { config: CRMProviderConfig; patch: ConfigPatch; disabled?: boolean }) {
    const crm = usePageCrm();
    return (
        <ToggleSetting
            title={`Write Warmbly ${crm.words.properties} to ${crm.name}`}
            description={
                crm.id === "pipedrive"
                    ? "Adds Warmbly fields to Pipedrive people: Warmbly status, last campaign, last contacted, last replied, last opened, last clicked, reply intent and an Open in Warmbly link. Use them in Pipedrive filters, reports and automations. Adding the fields needs a Pipedrive admin or the contact fields permission."
                    : "Adds a Warmbly property group to HubSpot contacts: Warmbly status, last campaign, last contacted, last replied, last opened, last clicked, reply intent, unsubscribed and an Open in Warmbly link. Use them in HubSpot views, lists and workflows."
            }
            value={config.write_properties}
            onChange={(v) => patch((c) => ({ ...c, write_properties: v }))}
            disabled={disabled}
        />
    );
}

// --- rules ---------------------------------------------------------------------

function firstOpenStage(p?: Pipeline) {
    return [...(p?.stages ?? [])].sort((a, b) => a.position - b.position).find((s) => !s.closed) ?? p?.stages?.[0];
}

export function ReplyOutcomeEditor({
    config,
    patch,
    metadata,
    disabled,
}: {
    config: CRMProviderConfig;
    patch: ConfigPatch;
    metadata?: CRMMetadata;
    disabled?: boolean;
}) {
    const crm = usePageCrm();
    const pd = crm.id === "pipedrive";
    const r = config.positive_reply;
    const { pipelines, loading } = useProviderPipelines();
    const pipeline = pipelines.find((p) => p.id === r.deal_pipeline_id);
    const stages = [...(pipeline?.stages ?? [])].sort((a, b) => a.position - b.position);

    const leadOptions: SelectOption[] = [
        { value: "", label: "Don't change" },
        ...withStoredValues(metadata?.lead_statuses, r.lead_status ? [r.lead_status] : []),
    ];
    const stageOptions: SelectOption[] = [
        { value: "", label: "Don't change" },
        ...withStoredValues(metadata?.lifecycle_stages, r.lifecycle_stage ? [r.lifecycle_stage] : []),
    ];
    const lifecycleLabel = pd ? "Label the person" : crm.words.lifecycle;

    const setReply = (next: Partial<CRMProviderConfig["positive_reply"]>) =>
        patch((c) => ({ ...c, positive_reply: { ...c.positive_reply, ...next } }));

    function toggleDeal(on: boolean) {
        if (!on) return setReply({ create_deal: false });
        const p = pipeline ?? pipelines[0];
        setReply({ create_deal: true, deal_pipeline_id: p?.id, deal_stage_id: r.deal_stage_id && pipeline ? r.deal_stage_id : firstOpenStage(p)?.id });
    }

    const noPipelines = !loading && pipelines.length === 0;

    return (
        <div className="space-y-3.5">
            <div className="grid sm:grid-cols-2 gap-3">
                {crm.words.leadStatus && (
                    <div>
                        <SubLabel className="mb-1.5">{crm.words.leadStatus}</SubLabel>
                        <SelectMenu
                            value={r.lead_status}
                            onChange={(v) => setReply({ lead_status: v })}
                            options={leadOptions}
                            fullWidth
                            disabled={disabled}
                            aria-label={`${crm.words.leadStatus} after a positive reply`}
                        />
                    </div>
                )}
                <div>
                    <SubLabel className="mb-1.5">{lifecycleLabel}</SubLabel>
                    <SelectMenu
                        value={r.lifecycle_stage}
                        onChange={(v) => setReply({ lifecycle_stage: v })}
                        options={stageOptions}
                        fullWidth
                        disabled={disabled}
                        aria-label={`${lifecycleLabel} after a positive reply`}
                    />
                </div>
            </div>
            {pd && (
                <ToggleSetting
                    title="Add to the Leads Inbox"
                    description="Adds a Pipedrive lead for the person and their organization, owned by whoever owns the mailbox that got the reply, so it can be qualified before it becomes a deal. Skipped when they already have an open deal."
                    value={!!r.create_lead}
                    onChange={(v) => setReply({ create_lead: v })}
                    disabled={disabled}
                />
            )}
            <ToggleSetting
                title="Create a deal"
                description={
                    noPipelines
                        ? `Your ${crm.name} pipelines appear here after the first sync. Come back to this once they have.`
                        : `Opens a ${crm.name} deal for the ${crm.words.contact}, assigned to whoever owns the mailbox that got the reply. Skipped when they already have an open deal.`
                }
                value={r.create_deal}
                onChange={toggleDeal}
                disabled={disabled || (noPipelines && !r.create_deal)}
            />
            {r.create_deal && (
                <div className="grid sm:grid-cols-2 gap-3 pl-0 sm:pl-3 sm:border-l-2 sm:border-slate-100">
                    <div>
                        <SubLabel className="mb-1.5">Deal pipeline</SubLabel>
                        <SelectMenu
                            value={r.deal_pipeline_id ?? ""}
                            onChange={(v) => {
                                const p = pipelines.find((x) => x.id === v);
                                setReply({ deal_pipeline_id: v, deal_stage_id: firstOpenStage(p)?.id });
                            }}
                            options={pipelines.map((p) => ({ value: p.id, label: p.name }))}
                            placeholder="Choose a pipeline"
                            fullWidth
                            disabled={disabled}
                            aria-label="Deal pipeline"
                        />
                    </div>
                    <div>
                        <SubLabel className="mb-1.5">Deal stage</SubLabel>
                        <SelectMenu
                            value={r.deal_stage_id ?? ""}
                            onChange={(v) => setReply({ deal_stage_id: v })}
                            options={stages.map((s) => ({ value: s.id, label: s.closed ? `${s.name} (${s.won ? "won" : "lost"})` : s.name }))}
                            placeholder={pipeline ? "Choose a stage" : "Choose a pipeline first"}
                            fullWidth
                            disabled={disabled || !pipeline}
                            aria-label="Deal stage"
                        />
                    </div>
                </div>
            )}
        </div>
    );
}

export function ExitRulesEditor({
    config,
    patch,
    metadata,
    disabled,
}: {
    config: CRMProviderConfig;
    patch: ConfigPatch;
    metadata?: CRMMetadata;
    disabled?: boolean;
}) {
    const crm = usePageCrm();
    const pd = crm.id === "pipedrive";
    const e = config.exit_rules;
    const set = (next: Partial<CRMProviderConfig["exit_rules"]>) => patch((c) => ({ ...c, exit_rules: { ...c.exit_rules, ...next } }));
    return (
        <div className="space-y-3.5">
            <ToggleSetting
                title="A deal is created for them"
                description={`Someone on your team opened a deal in ${crm.name}, so the conversation has moved on.`}
                value={e.deal_created}
                onChange={(v) => set({ deal_created: v })}
                disabled={disabled}
            />
            <SettingRow
                title={pd ? "They get one of these labels" : "Their lifecycle stage becomes"}
                description={pd ? "Leave empty to ignore label changes." : "Leave empty to ignore lifecycle changes."}
                stack
                control={
                    <MultiPicker
                        options={withStoredValues(metadata?.lifecycle_stages, e.lifecycle_stages)}
                        selected={e.lifecycle_stages}
                        onChange={(v) => set({ lifecycle_stages: v })}
                        placeholder={`Choose ${crm.words.lifecycles}`}
                        searchPlaceholder={`Search ${crm.words.lifecycles}…`}
                        disabled={disabled}
                        ariaLabel={`${crm.words.lifecycles} that stop a campaign`}
                    />
                }
            />
            <ToggleSetting
                title={pd ? "They unsubscribe in Pipedrive" : "They opt out of email in HubSpot"}
                description={
                    pd
                        ? "An unsubscribe recorded by Pipedrive Campaigns stops Warmbly too."
                        : "An unsubscribe recorded in HubSpot stops Warmbly too."
                }
                value={e.opted_out}
                onChange={(v) => set({ opted_out: v })}
                disabled={disabled}
            />
        </div>
    );
}

export function GuardsEditor({
    config,
    patch,
    metadata,
    disabled,
}: {
    config: CRMProviderConfig;
    patch: ConfigPatch;
    metadata?: CRMMetadata;
    disabled?: boolean;
}) {
    const crm = usePageCrm();
    const pd = crm.id === "pipedrive";
    const g = config.guards;
    const set = (next: Partial<CRMProviderConfig["guards"]>) => patch((c) => ({ ...c, guards: { ...c.guards, ...next } }));
    return (
        <div className="space-y-3.5">
            <SettingRow
                title={pd ? "Have one of these labels" : "Are in these lifecycle stages"}
                description={
                    pd
                        ? "People labeled Customer are skipped by default so nobody cold-emails a happy customer."
                        : "Customers and evangelists are skipped by default so nobody cold-emails a happy customer."
                }
                stack
                control={
                    <MultiPicker
                        options={withStoredValues(metadata?.lifecycle_stages, g.skip_lifecycle_stages)}
                        selected={g.skip_lifecycle_stages}
                        onChange={(v) => set({ skip_lifecycle_stages: v })}
                        placeholder={`Choose ${crm.words.lifecycles}`}
                        searchPlaceholder={`Search ${crm.words.lifecycles}…`}
                        disabled={disabled}
                        ariaLabel={`${crm.words.lifecycles} to skip`}
                    />
                }
            />
            <ToggleSetting
                title="Have an open deal"
                description="Someone is already working them."
                value={g.skip_open_deals}
                onChange={(v) => set({ skip_open_deals: v })}
                disabled={disabled}
            />
            <ToggleSetting
                title="Are owned by someone outside this workspace"
                description={`Their ${crm.name} ${crm.words.owner} is not matched to a workspace member.`}
                value={g.skip_other_owners}
                onChange={(v) => set({ skip_other_owners: v })}
                disabled={disabled}
            />
            <ToggleSetting
                title={pd ? "Unsubscribed in Pipedrive" : "Opted out of email in HubSpot"}
                description="Respects unsubscribes collected outside Warmbly."
                value={g.skip_opted_out}
                onChange={(v) => set({ skip_opted_out: v })}
                disabled={disabled}
            />
        </div>
    );
}

// --- pipelines and display properties ----------------------------------------------

export function PipelinesEditor({
    config,
    patch,
    metadata,
    disabled,
}: {
    config: CRMProviderConfig;
    patch: ConfigPatch;
    metadata?: CRMMetadata;
    disabled?: boolean;
}) {
    const crm = usePageCrm();
    return (
        <MultiPicker
            options={withStoredValues(metadata?.pipelines, config.deal_pipelines)}
            selected={config.deal_pipelines}
            onChange={(v) => patch((c) => ({ ...c, deal_pipelines: v }))}
            emptyLabel="All pipelines"
            searchPlaceholder="Search pipelines…"
            disabled={disabled}
            ariaLabel={`${crm.name} pipelines to mirror`}
        />
    );
}

export function DisplayPropertiesEditor({
    config,
    patch,
    properties,
    disabled,
}: {
    config: CRMProviderConfig;
    patch: ConfigPatch;
    properties: CRMProperty[];
    disabled?: boolean;
}) {
    const crm = usePageCrm();
    const options = React.useMemo(() => {
        // Pipedrive shows its standard person keys on the panel already.
        const shown = crm.id === "pipedrive" ? properties.filter((p) => p.group_name !== "Person") : properties;
        const out = shown.map((p) => ({ value: p.name, label: p.label, hint: p.name }));
        for (const v of config.display_properties) if (!out.some((o) => o.value === v)) out.push({ value: v, label: humanize(v), hint: v });
        return out;
    }, [properties, config.display_properties, crm.id]);
    return (
        <MultiPicker
            options={options}
            selected={config.display_properties}
            onChange={(v) => patch((c) => ({ ...c, display_properties: v.slice(0, 40) }))}
            placeholder={`Choose ${crm.name} ${crm.words.properties}`}
            searchPlaceholder={`Search ${crm.name} ${crm.words.properties}…`}
            disabled={disabled}
            ariaLabel={`${crm.name} ${crm.words.properties} shown in Warmbly`}
        />
    );
}

// --- owners --------------------------------------------------------------------

const OWNER_WAIT_MS = 90_000;

function ownerName(o: CRMOwner) {
    return [o.first_name, o.last_name].filter(Boolean).join(" ") || o.email || "User";
}

// The CRM's users matched to workspace members. Owners arrive with the first
// pull after the switch, so `waiting` keeps asking until they do.
export function OwnersTable({ disabled, waiting = false }: { disabled?: boolean; waiting?: boolean }) {
    const crm = usePageCrm();
    const queryClient = useQueryClient();
    const [gaveUp, setGaveUp] = React.useState(false);
    React.useEffect(() => {
        if (!waiting) return;
        const t = setTimeout(() => setGaveUp(true), OWNER_WAIT_MS);
        return () => clearTimeout(t);
    }, [waiting]);

    const owners = useQuery({
        queryKey: ["crm", "owners"],
        queryFn: async () => (await listCrmOwners()).data,
        staleTime: 60_000,
        refetchInterval: (q) => (waiting && !gaveUp && !(q.state.data?.length ?? 0) ? 3000 : false),
    });
    const members = useMembers();
    const map = useMapCrmOwner();

    // A pull that changed the owner count refreshes the list (CRM_SYNCED keeps health live).
    const ownerCount = useCrmSyncHealth().data?.counts.owners;
    const seenCount = React.useRef<number | undefined>(undefined);
    React.useEffect(() => {
        if (ownerCount === undefined) return;
        const prev = seenCount.current;
        seenCount.current = ownerCount;
        if (prev !== undefined && prev !== ownerCount) void queryClient.invalidateQueries({ queryKey: ["crm", "owners"] });
    }, [ownerCount, queryClient]);

    const list = (owners.data ?? []).filter((o) => !o.archived);
    const archived = (owners.data ?? []).length - list.length;
    const memberOptions: SelectOption[] = [
        { value: "", label: "Not a workspace member" },
        ...(members.data ?? []).map((m) => ({ value: m.user_id, label: m.name || m.email || "Member" })),
    ];

    if (owners.isLoading || (list.length === 0 && waiting && !gaveUp)) {
        return (
            <div className="rounded-md border border-slate-200 px-4 py-6 flex flex-col items-center gap-2 text-center">
                <Loader2Icon className="w-4 h-4 text-slate-400 animate-spin" />
                <p className="text-[12.5px] text-slate-700">Fetching your {crm.name} users…</p>
                <p className="text-[11px] text-slate-400">This usually takes a few seconds. You can continue and match them later.</p>
            </div>
        );
    }
    if (owners.isError) {
        return <p className="text-[12px] text-rose-600">{errMessage(owners.error, `Could not load ${crm.name} users.`)}</p>;
    }
    if (list.length === 0) {
        return (
            <p className="rounded-md border border-slate-200 px-4 py-5 text-[12px] text-slate-500 text-center">
                No {crm.name} users yet. They show up here after the first sync, and you can match them then.
            </p>
        );
    }

    return (
        <div className="space-y-2">
            <div className="rounded-md border border-slate-200 divide-y divide-slate-100">
                {list.map((o) => {
                    const status = o.user_id
                        ? o.user_pinned
                            ? { label: "Set by you", cls: "bg-sky-50 text-sky-700" }
                            : { label: "Matched by email", cls: "bg-emerald-50 text-emerald-700" }
                        : { label: "No match", cls: "bg-slate-100 text-slate-500" };
                    return (
                        <div key={o.external_id} className="px-3 py-2 flex flex-col sm:flex-row sm:items-center gap-2">
                            <div className="flex items-center gap-2.5 min-w-0 flex-1">
                                <span
                                    className={cn(
                                        "size-6 rounded-full text-[10px] font-semibold inline-flex items-center justify-center shrink-0",
                                        crm.tint,
                                        crm.tintText,
                                    )}
                                >
                                    {ownerName(o).slice(0, 1).toUpperCase()}
                                </span>
                                <div className="min-w-0">
                                    <div className="text-[12.5px] text-slate-900 truncate">{ownerName(o)}</div>
                                    <div className="text-[11px] text-slate-400 truncate">{o.email}</div>
                                </div>
                                <span className={cn("ml-auto sm:ml-0 h-5 px-1.5 rounded text-[10.5px] font-medium inline-flex items-center shrink-0", status.cls)}>
                                    {status.label}
                                </span>
                            </div>
                            <div className="sm:w-56 shrink-0">
                                <SelectMenu
                                    value={o.user_id ?? ""}
                                    onChange={(v) =>
                                        map.mutate(
                                            { externalId: o.external_id, userId: v || null },
                                            { onError: (err) => toast.error(errMessage(err, "Could not save the match")) },
                                        )
                                    }
                                    options={memberOptions}
                                    fullWidth
                                    disabled={disabled}
                                    aria-label={`Workspace member for ${ownerName(o)}`}
                                />
                            </div>
                        </div>
                    );
                })}
            </div>
            {archived > 0 && (
                <p className="text-[11px] text-slate-400">
                    {archived} deactivated {crm.name} {archived === 1 ? "user is" : "users are"} hidden.
                </p>
            )}
        </div>
    );
}
