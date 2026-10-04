// Sync rules: who gets matched or created in Salesforce, what lands on the
// timeline, which Lead statuses Warmbly writes, and what Salesforce changes do
// to outreach. Edits the shared settings draft; the page owns the save bar.

import React from "react";
import { AlertTriangleIcon, InfoIcon } from "lucide-react";

import { NumberInput, TextInput } from "@/components/ui/field";
import { SelectMenu } from "@/components/ui/select-menu";
import {
    OptionSelect,
    Segmented,
    Toggle,
} from "@/components/app/campaigns/preferences/components/CampaignPreferenceBoolBox";
import { useSalesforceMetadata, useSalesforceUsers } from "@/lib/api/hooks/app/integrations/useSalesforce";
import {
    SALESFORCE_ACTIVITY_KINDS,
    SALESFORCE_ACTIVITY_LABELS,
    SALESFORCE_REPLY_INTENTS,
    type SalesforceActivityKind,
    type SalesforceSettings,
} from "@/lib/api/models/app/integrations/Salesforce";

import { CardRow, ChipMultiPicker, PicklistSelect, SearchSelect, SettingsCard, ToggleRow } from "./shared";
import { errMsg } from "./util";

type Patch = (fn: (s: SalesforceSettings) => SalesforceSettings) => void;

const ACTIVITY_HINTS: Partial<Record<SalesforceActivityKind, string>> = {
    sent: "A completed Task for every campaign email.",
    replied: "The reply, with its intent, as a completed Task.",
    meeting_booked: "Booked through Calendly, Cal.com or added in Warmbly.",
    bounced: "So reps know the address is dead.",
    unsubscribed: "Logged alongside the opt-out flag.",
    opened: "Costs one API call per open and clutters the timeline.",
    clicked: "Costs one API call per click and clutters the timeline.",
};

export default function SyncRulesTab({
    connectionId,
    draft,
    patch,
}: {
    connectionId: string;
    draft: SalesforceSettings;
    patch: Patch;
}) {
    const meta = useSalesforceMetadata(connectionId);
    const statuses = meta.data?.lead_statuses ?? [];
    const sources = meta.data?.lead_sources ?? [];

    const m = draft.matching;
    const a = draft.activity;
    const w = draft.writeback;
    const inb = draft.inbound;
    const createNoun = m.create_as === "contact" ? "Contact" : "Lead";

    const setMatching = (p: Partial<SalesforceSettings["matching"]>) =>
        patch((s) => ({ ...s, matching: { ...s.matching, ...p } }));
    const setActivity = (p: Partial<SalesforceSettings["activity"]>) =>
        patch((s) => ({ ...s, activity: { ...s.activity, ...p } }));
    const setWriteback = (p: Partial<SalesforceSettings["writeback"]>) =>
        patch((s) => ({ ...s, writeback: { ...s.writeback, ...p } }));
    const setInbound = (p: Partial<SalesforceSettings["inbound"]>) =>
        patch((s) => ({ ...s, inbound: { ...s.inbound, ...p } }));

    function setReplyStatus(intent: string, value: string) {
        patch((s) => {
            const next = { ...s.writeback.lead_status_on_reply };
            if (value) next[intent] = value;
            else delete next[intent];
            return { ...s, writeback: { ...s.writeback, lead_status_on_reply: next } };
        });
    }

    return (
        <div className="space-y-6 max-w-3xl">
            {meta.isError && (
                <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 flex items-start gap-2 text-[11.5px] text-amber-800">
                    <AlertTriangleIcon className="w-3.5 h-3.5 mt-0.5 shrink-0" />
                    <span>
                        Could not read your Salesforce picklists ({errMsg(meta.error, "unknown error")}). Lead status
                        choices are limited to values already saved.
                    </span>
                </div>
            )}

            <div className="rounded-md border border-slate-200 bg-white px-4 py-3">
                <ToggleRow
                    title="Sync with Salesforce"
                    description="Master switch. Off stops activity logging, status writeback and reading changes back. Imports keep working."
                    control={
                        <Toggle
                            value={draft.enabled}
                            onChange={(v) => patch((s) => ({ ...s, enabled: v }))}
                            ariaLabel="Sync with Salesforce"
                        />
                    }
                />
            </div>

            <SettingsCard
                title="Matching"
                description="How a Warmbly contact finds its Salesforce record. Matching is by email address."
            >
                <CardRow>
                    <ToggleRow
                        title="When both a Lead and a Contact match"
                        description="Which record the activity is logged against."
                        control={
                            <Segmented
                                value={m.prefer}
                                onChange={(v) => setMatching({ prefer: v })}
                                options={[
                                    { value: "contact", label: "Contact" },
                                    { value: "lead", label: "Lead" },
                                ]}
                            />
                        }
                    />
                </CardRow>
                <CardRow className="space-y-2">
                    <div className="text-[12.5px] text-slate-900 font-medium">When nobody matches</div>
                    <OptionSelect
                        value={m.create_when}
                        onChange={(v) => setMatching({ create_when: v })}
                        aria-label="When nobody matches"
                        options={[
                            { value: "never", label: "Only log activity for people already in Salesforce", hint: "Nothing is created." },
                            {
                                value: "reply",
                                label: `Create a ${createNoun} when someone replies or books a meeting`,
                                hint: "Recommended. Salesforce only gets people who engaged.",
                            },
                            { value: "send", label: `Create a ${createNoun} on first email`, hint: "Every contacted person lands in Salesforce." },
                        ]}
                    />
                </CardRow>
                {m.create_when !== "never" && (
                    <>
                        <CardRow>
                            <ToggleRow
                                title="Create as"
                                control={
                                    <Segmented
                                        value={m.create_as}
                                        onChange={(v) => setMatching({ create_as: v })}
                                        options={[
                                            { value: "lead", label: "Lead" },
                                            { value: "contact", label: "Contact" },
                                        ]}
                                    />
                                }
                            />
                        </CardRow>
                        <CardRow className="grid gap-3 sm:grid-cols-2">
                            <div>
                                <FieldLabel>Lead source</FieldLabel>
                                <TextInput
                                    value={m.lead_source}
                                    onChange={(v) => setMatching({ lead_source: v })}
                                    placeholder="Warmbly"
                                    maxLength={120}
                                    className="w-full"
                                />
                                <p className="text-[10.5px] text-slate-400 mt-1 leading-relaxed">
                                    {sources.length > 0
                                        ? `Must match a Lead Source value if the picklist is restricted, such as ${sources
                                              .slice(0, 3)
                                              .map((v) => v.value)
                                              .join(", ")}.`
                                        : "Must match a Lead Source value if the picklist is restricted."}
                                </p>
                            </div>
                            {m.create_as === "lead" && (
                                <div>
                                    <FieldLabel>Initial Lead status</FieldLabel>
                                    <PicklistSelect
                                        value={m.lead_status}
                                        onChange={(v) => setMatching({ lead_status: v })}
                                        values={statuses}
                                        emptyLabel="Salesforce default"
                                        loading={meta.isPending}
                                        className="w-full"
                                        aria-label="Initial Lead status"
                                    />
                                </div>
                            )}
                        </CardRow>
                        <CardRow className="space-y-2">
                            <div className="text-[12.5px] text-slate-900 font-medium">Owner of created records</div>
                            <OptionSelect
                                value={m.owner}
                                onChange={(v) => setMatching({ owner: v })}
                                aria-label="Owner of created records"
                                options={[
                                    { value: "connected_user", label: "The connected Salesforce user" },
                                    { value: "sender", label: "Salesforce user with the sending mailbox's email", hint: "Falls back to the connected user." },
                                    { value: "fixed", label: "A specific user" },
                                ]}
                            />
                            {m.owner === "fixed" && (
                                <OwnerPicker
                                    connectionId={connectionId}
                                    value={m.owner_id ?? ""}
                                    onChange={(id) => setMatching({ owner_id: id })}
                                />
                            )}
                        </CardRow>
                        <CardRow>
                            <ToggleRow
                                title="Run assignment rules"
                                description="Let your Lead assignment rules pick the owner instead. Applies to Leads only."
                                control={
                                    <Toggle
                                        value={m.run_assignment_rules}
                                        onChange={(v) => setMatching({ run_assignment_rules: v })}
                                        ariaLabel="Run assignment rules"
                                    />
                                }
                            />
                        </CardRow>
                    </>
                )}
            </SettingsCard>

            <SettingsCard
                title="Activity logging"
                description="Each event becomes a completed Task on the matched Lead or Contact."
            >
                {SALESFORCE_ACTIVITY_KINDS.map((k) => (
                    <CardRow key={k} className="py-2.5">
                        <ToggleRow
                            title={SALESFORCE_ACTIVITY_LABELS[k]}
                            description={ACTIVITY_HINTS[k]}
                            control={
                                <Toggle
                                    value={a[k]}
                                    onChange={(v) => setActivity({ [k]: v } as Partial<SalesforceSettings["activity"]>)}
                                    ariaLabel={`Log ${SALESFORCE_ACTIVITY_LABELS[k]}`}
                                />
                            }
                        />
                    </CardRow>
                ))}
                <CardRow className="py-2 bg-slate-50/60">
                    <p className="text-[11px] text-slate-500 flex items-start gap-1.5">
                        <InfoIcon className="w-3 h-3 mt-0.5 shrink-0 text-slate-400" />
                        Opens and clicks are off by default: each one costs an API call, machine opens inflate them, and
                        they bury the conversations reps care about.
                    </p>
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="Include the email body"
                        description="Puts the message text in the Task description. Off logs the subject only."
                        control={
                            <Toggle value={a.include_body} onChange={(v) => setActivity({ include_body: v })} ariaLabel="Include the email body" />
                        }
                    />
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="Assign Tasks to"
                        control={
                            <SelectMenu
                                value={a.assign_to}
                                onChange={(v) => setActivity({ assign_to: v as SalesforceSettings["activity"]["assign_to"] })}
                                minWidth={260}
                                aria-label="Assign Tasks to"
                                options={[
                                    { value: "record_owner", label: "The record's owner" },
                                    { value: "sender", label: "The sender's Salesforce user" },
                                    { value: "connected_user", label: "The connected user" },
                                ]}
                            />
                        }
                    />
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="Relate to the open Opportunity"
                        description="When the Contact's Account has one open Opportunity, the Task is related to it too."
                        control={
                            <Toggle
                                value={a.relate_to_opportunity}
                                onChange={(v) => setActivity({ relate_to_opportunity: v })}
                                ariaLabel="Relate to the open Opportunity"
                            />
                        }
                    />
                </CardRow>
            </SettingsCard>

            <SettingsCard
                title="Lead status writeback"
                description="Moves a Lead's status as outreach progresses. Empty means Warmbly leaves the status alone."
            >
                <CardRow>
                    <ToggleRow
                        title="After the first email"
                        control={
                            <PicklistSelect
                                value={w.lead_status_on_sent}
                                onChange={(v) => setWriteback({ lead_status_on_sent: v })}
                                values={statuses}
                                loading={meta.isPending}
                                className="w-56"
                                aria-label="Status after the first email"
                            />
                        }
                    />
                </CardRow>
                <CardRow className="space-y-2">
                    <div>
                        <div className="text-[12.5px] text-slate-900 font-medium">When they reply</div>
                        <p className="text-[11px] text-slate-500 mt-0.5">
                            A specific intent wins over “Any reply”. Out-of-office replies only move a status you set for them.
                        </p>
                    </div>
                    <div className="rounded-md border border-slate-200 divide-y divide-slate-100">
                        {SALESFORCE_REPLY_INTENTS.map((intent) => (
                            <div key={intent.value} className="flex items-center justify-between gap-3 px-3 py-1.5">
                                <span className="text-[12px] text-slate-700">{intent.label}</span>
                                <PicklistSelect
                                    value={w.lead_status_on_reply[intent.value] ?? ""}
                                    onChange={(v) => setReplyStatus(intent.value, v)}
                                    values={statuses}
                                    loading={meta.isPending}
                                    className="w-56"
                                    aria-label={`Status on ${intent.label}`}
                                />
                            </div>
                        ))}
                    </div>
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="When a meeting is booked"
                        control={
                            <PicklistSelect
                                value={w.lead_status_on_meeting}
                                onChange={(v) => setWriteback({ lead_status_on_meeting: v })}
                                values={statuses}
                                loading={meta.isPending}
                                className="w-56"
                                aria-label="Status when a meeting is booked"
                            />
                        }
                    />
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="Never move a status backwards"
                        description="Uses the order of your Lead status picklist, so a Working lead is not reset to Contacted by a later email."
                        control={
                            <Toggle
                                value={w.never_move_backwards}
                                onChange={(v) => setWriteback({ never_move_backwards: v })}
                                ariaLabel="Never move a status backwards"
                            />
                        }
                    />
                </CardRow>
            </SettingsCard>

            <SettingsCard
                title="When Salesforce changes"
                description="Warmbly reads Lead and Contact changes back every few minutes."
            >
                <CardRow className="space-y-2">
                    <div>
                        <div className="text-[12.5px] text-slate-900 font-medium">Do-not-email sync</div>
                        <p className="text-[11px] text-slate-500 mt-0.5">
                            Keeps Salesforce's Email Opt Out (HasOptedOutOfEmail) and Warmbly's unsubscribes in step.
                        </p>
                    </div>
                    <OptionSelect
                        value={inb.opt_out}
                        onChange={(v) => setInbound({ opt_out: v })}
                        cols={2}
                        aria-label="Do-not-email sync"
                        options={[
                            { value: "both", label: "Both ways", hint: "Recommended" },
                            { value: "to_salesforce", label: "Warmbly to Salesforce only", hint: "Unsubscribes set Email Opt Out" },
                            { value: "from_salesforce", label: "Salesforce to Warmbly only", hint: "Email Opt Out unsubscribes here" },
                            { value: "off", label: "Off", hint: "Each side keeps its own list" },
                        ]}
                    />
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="Pause outreach when a Lead is converted"
                        description="The rep owns the relationship from there."
                        control={
                            <Toggle
                                value={inb.pause_on_converted}
                                onChange={(v) => setInbound({ pause_on_converted: v })}
                                ariaLabel="Pause outreach when a Lead is converted"
                            />
                        }
                    />
                </CardRow>
                <CardRow className="space-y-2">
                    <div>
                        <div className="text-[12.5px] text-slate-900 font-medium">Pause outreach at these Lead statuses</div>
                        <p className="text-[11px] text-slate-500 mt-0.5">
                            For example Unqualified, or a status your reps set when they take over.
                        </p>
                    </div>
                    <ChipMultiPicker
                        value={inb.pause_on_statuses}
                        onChange={(v) => setInbound({ pause_on_statuses: v })}
                        options={statuses}
                        placeholder={meta.isPending ? "Loading statuses…" : "Click to add statuses…"}
                    />
                </CardRow>
                <CardRow>
                    <ToggleRow
                        title="Pause outreach when an Opportunity is open"
                        description="Stops sequences to Contacts whose Account has an open Opportunity."
                        control={
                            <Toggle
                                value={inb.pause_on_open_opportunity}
                                onChange={(v) => setInbound({ pause_on_open_opportunity: v })}
                                ariaLabel="Pause outreach when an Opportunity is open"
                            />
                        }
                    />
                </CardRow>
            </SettingsCard>

            <SettingsCard title="API budget">
                <CardRow>
                    <ToggleRow
                        title="Daily API calls Warmbly may use"
                        description="0 is automatic: a fifth of your org's daily allocation. Sync waits until tomorrow once the budget is spent."
                        control={
                            <NumberInput
                                value={draft.daily_api_budget}
                                onChange={(v) => patch((s) => ({ ...s, daily_api_budget: v }))}
                                min={0}
                                max={1_000_000}
                                step={500}
                                suffix="calls / day"
                                className="w-44"
                            />
                        }
                    />
                </CardRow>
            </SettingsCard>
        </div>
    );
}

function FieldLabel({ children }: { children: React.ReactNode }) {
    return <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium mb-1.5">{children}</div>;
}

// Picks the Salesforce user that owns created records, searched server-side.
function OwnerPicker({
    connectionId,
    value,
    onChange,
}: {
    connectionId: string;
    value: string;
    onChange: (id: string) => void;
}) {
    const [q, setQ] = React.useState("");
    const [debounced, setDebounced] = React.useState("");
    React.useEffect(() => {
        const t = window.setTimeout(() => setDebounced(q.trim()), 250);
        return () => window.clearTimeout(t);
    }, [q]);
    const users = useSalesforceUsers(connectionId, debounced);
    const [picked, setPicked] = React.useState<{ id: string; label: string } | null>(null);

    const options = (users.data ?? []).map((u) => ({ value: u.id, label: u.name, hint: u.email }));
    const label = picked?.id === value ? picked.label : value;

    return (
        <div className="space-y-1">
            <SearchSelect
                value={value}
                valueLabel={label}
                onChange={(id, o) => {
                    setPicked({ id, label: o.label });
                    onChange(id);
                }}
                options={options}
                onQueryChange={setQ}
                loading={users.isFetching}
                placeholder="Choose a Salesforce user"
                searchPlaceholder="Search by name or email…"
                emptyText={users.isError ? errMsg(users.error, "Could not search users") : "No users match."}
                className="w-full sm:w-80"
                aria-label="Owner"
            />
            {!value && <p className="text-[11px] text-amber-700">Choose a user, or the settings cannot be saved.</p>}
        </div>
    );
}
