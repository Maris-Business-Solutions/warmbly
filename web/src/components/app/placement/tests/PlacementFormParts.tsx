// Form pieces shared by the new placement test and new placement batch
// dialogs: the copy to test, tracking, pace, seed panel and the pickers they use.

import React from "react";
import { AlertCircleIcon, Loader2Icon, MegaphoneIcon, UserRoundIcon } from "lucide-react";
import { Label, SearchInput, TextInput } from "@/components/ui/field";
import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuItem,
    PopoverMenuLabel,
    PopoverMenuSeparator,
    PopoverMenuTrigger,
    SelectButton,
} from "@/components/ui/popover-menu";
import { SelectMenu } from "@/components/ui/select-menu";
import { OptionSelect, Segmented } from "@/components/app/campaigns/preferences/components/CampaignPreferenceBoolBox";
import RichTextEditor from "@/components/app/campaigns/sequences/RichTextEditor";
import { VARIABLES, htmlToPlain } from "@/components/app/campaigns/sequences/emailPreview";
import { contactLabel } from "@/components/app/campaigns/sequences/previewContext";
import { LINK_VARIABLES } from "@/lib/templateVars";
import useDebouncedValue from "@/hooks/useDebouncedValue";
import useCampaigns from "@/lib/api/hooks/app/campaigns/useCampaigns";
import useSearchContacts from "@/lib/api/hooks/app/contacts/useSearchContacts";
import {
    PANEL_LABEL,
    type PlacementPace,
    type PlacementPanel,
    type PlacementPanelFamily,
    type PlacementPanelInfo,
    type PlacementTracking,
    type PlacementUsage,
} from "@/lib/api/models/app/placement/Placement";
import type Contact from "@/lib/api/models/app/contacts/Contact";
import { cn } from "@/lib/utils";
import type { CopyDraft, CopySource } from "./placementCopy";

export function SectionLabel({ children }: { children: React.ReactNode }) {
    return <span className="block mb-2 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{children}</span>;
}

export function CopySourceFields({
    value,
    patch,
    onSource,
    campaignName,
    steps,
    error,
}: {
    value: CopyDraft;
    patch: (p: Partial<CopyDraft>) => void;
    onSource: (s: CopySource) => void;
    campaignName?: string;
    steps: { emailSteps: { id: string; name?: string; subject?: string }[]; isLoading: boolean };
    error?: React.ReactNode;
}) {
    const { emailSteps } = steps;
    return (
        <section className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">What to test</span>
                <Segmented<CopySource>
                    value={value.source}
                    onChange={onSource}
                    options={[
                        { value: "step", label: "Campaign step" },
                        { value: "custom", label: "Custom copy" },
                    ]}
                />
            </div>

            {value.source === "step" ? (
                <div className="grid gap-3 sm:grid-cols-2">
                    <div className="min-w-0">
                        <Label>Campaign</Label>
                        <CampaignPicker
                            value={value.campaignId}
                            name={campaignName}
                            onChange={(id) => patch({ campaignId: id, stepId: "", contact: null })}
                        />
                    </div>
                    <div className="min-w-0">
                        <Label>Step</Label>
                        <SelectMenu
                            value={value.stepId}
                            onChange={(v) => patch({ stepId: v })}
                            disabled={!value.campaignId || steps.isLoading}
                            fullWidth
                            placeholder={
                                !value.campaignId
                                    ? "Pick a campaign first"
                                    : steps.isLoading
                                      ? "Loading steps…"
                                      : emailSteps.length === 0
                                        ? "No email steps"
                                        : "Pick a step"
                            }
                            options={emailSteps.map((s, i) => ({
                                value: s.id,
                                label: `${s.name || `Step ${i + 1}`}${s.subject ? `: ${s.subject}` : ""}`,
                            }))}
                            aria-label="Step"
                        />
                    </div>
                    <p className="sm:col-span-2 text-[11px] text-slate-400 leading-relaxed">
                        The saved step is rendered exactly as the campaign sends it: merge fields, spintax,
                        signature, opt-out footer and unsubscribe header.
                    </p>
                </div>
            ) : (
                <div className="space-y-3">
                    <div>
                        <Label>Subject</Label>
                        <TextInput
                            value={value.subject}
                            onChange={(v) => patch({ subject: v })}
                            placeholder="Quick question, {{.FirstName}}"
                        />
                    </div>
                    <div>
                        <Label>Body</Label>
                        <RichTextEditor
                            html={value.bodyHtml}
                            onChange={(html) => patch({ bodyHtml: html, bodyPlain: value.bodyCode ? "" : htmlToPlain(html) })}
                            code={value.bodyCode}
                            onCodeChange={(c) => patch({ bodyCode: c })}
                            variables={VARIABLES}
                            links={LINK_VARIABLES}
                            placeholder="Hi {{.FirstName}}, …"
                        />
                    </div>
                </div>
            )}
            {error}

            <div>
                <Label>Render for</Label>
                <ContactPicker
                    campaignId={value.source === "step" ? value.campaignId : ""}
                    value={value.contact}
                    onChange={(c) => patch({ contact: c })}
                />
                <p className="mt-1.5 text-[11px] text-slate-400 leading-relaxed">
                    Fills the merge fields. Nobody but the seed inboxes receives the copies.
                </p>
            </div>
        </section>
    );
}

export function TrackingChoice({
    value,
    onChange,
    source,
    textOnly,
    compareHint = "Two tests to the same seeds. Counts as 2 tests.",
    error,
}: {
    value: PlacementTracking;
    onChange: (v: PlacementTracking) => void;
    source: CopySource;
    textOnly: boolean;
    compareHint?: string;
    error?: React.ReactNode;
}) {
    return (
        <section>
            <SectionLabel>Tracking</SectionLabel>
            <OptionSelect<PlacementTracking>
                value={value}
                onChange={onChange}
                cols={2}
                aria-label="Tracking"
                options={[
                    ...(source === "step"
                        ? [{ value: "campaign" as const, label: "As the campaign", hint: "Uses the campaign's open and click tracking." }]
                        : []),
                    ...(textOnly ? [] : [{ value: "on" as const, label: "On", hint: "Open pixel and tracked links." }]),
                    { value: "off" as const, label: "Off", hint: "No pixel, links left as written." },
                    ...(textOnly ? [] : [{ value: "compare" as const, label: "Compare with and without", hint: compareHint }]),
                ]}
            />
            {textOnly && <p className="mt-1.5 text-[11px] text-slate-400">This campaign sends plain text, which carries no tracking.</p>}
            {error}
        </section>
    );
}

export function PaceChoice({ value, onChange }: { value: PlacementPace; onChange: (v: PlacementPace) => void }) {
    return (
        <section>
            <SectionLabel>Pace</SectionLabel>
            <OptionSelect<PlacementPace>
                value={value}
                onChange={onChange}
                cols={2}
                aria-label="Pace"
                options={[
                    { value: "spaced", label: "Spaced", hint: "About a minute between copies, the way a campaign sends." },
                    { value: "quick", label: "Quick", hint: "A few seconds apart, so results come in within minutes." },
                ]}
            />
        </section>
    );
}

// The seed panels as radio cards; an unavailable one says why.
export function PanelChoice({
    panels,
    loading,
    value,
    onChange,
    usage,
}: {
    panels: PlacementPanelInfo[];
    loading: boolean;
    value: PlacementPanel;
    onChange: (p: PlacementPanel) => void;
    usage?: PlacementUsage;
}) {
    if (loading) return <div className="h-16 rounded-md bg-slate-50 animate-pulse" />;
    return (
        <div role="radiogroup" aria-label="Seed panel" className="grid gap-1.5">
            {panels.map((p) => {
                const active = p.panel === value;
                return (
                    <button
                        key={p.panel}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        disabled={!p.available}
                        onClick={() => onChange(p.panel)}
                        className={cn(
                            "flex w-full items-start gap-2.5 rounded-md border px-3 py-2 text-left transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sky-100",
                            active && p.available
                                ? "border-sky-300 bg-sky-50"
                                : "border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50",
                            !p.available && "opacity-60 cursor-not-allowed hover:bg-white hover:border-slate-200",
                        )}
                    >
                        <span className="min-w-0 flex-1">
                            <span className="flex items-center gap-2">
                                <span className={cn("text-[12px] font-medium", active && p.available ? "text-sky-700" : "text-slate-700")}>
                                    {PANEL_LABEL[p.panel]}
                                </span>
                                <span className="font-mono text-[10.5px] text-slate-400 tabular-nums">
                                    {p.seeds} seed{p.seeds === 1 ? "" : "s"}
                                </span>
                                {p.metered && p.available && (
                                    <span className="h-4 px-1.5 rounded bg-slate-100 text-[10px] text-slate-500 inline-flex items-center">
                                        Counted
                                    </span>
                                )}
                            </span>
                            <span className="mt-0.5 block text-[11px] leading-snug text-slate-400">
                                {!p.available
                                    ? p.reason || "Not available on this workspace."
                                    : p.metered
                                      ? usage?.limit != null
                                          ? `Counts toward your monthly tests: ${usage.used} of ${usage.limit} used.`
                                          : "Counts toward your monthly tests."
                                      : "Not counted toward your monthly tests."}
                            </span>
                        </span>
                        <span
                            className={cn(
                                "mt-0.5 size-4 shrink-0 rounded-full border transition-colors",
                                active && p.available ? "border-sky-600 bg-sky-600 ring-2 ring-inset ring-white" : "border-slate-300 bg-white",
                            )}
                            aria-hidden="true"
                        />
                    </button>
                );
            })}
        </div>
    );
}

// The shared panels' provider families; nothing picked tests every provider.
export function FamilyChips({
    families,
    value,
    onChange,
}: {
    families: PlacementPanelFamily[];
    value: string[];
    onChange: (families: string[]) => void;
}) {
    const chip = (active: boolean) =>
        cn(
            "h-6 px-2 rounded-md border text-[11px] font-medium inline-flex items-center gap-1 transition-colors",
            active ? "border-sky-200 bg-sky-50 text-sky-700" : "border-slate-200 bg-white text-slate-600 hover:border-slate-300",
        );
    return (
        <div className="mt-2">
            <span className="block mb-1.5 text-[11px] text-slate-500">Providers</span>
            <div className="flex flex-wrap gap-1">
                <button type="button" aria-pressed={value.length === 0} onClick={() => onChange([])} className={chip(value.length === 0)}>
                    All
                </button>
                {families.map((f) => {
                    const active = value.includes(f.family);
                    return (
                        <button
                            key={f.family}
                            type="button"
                            aria-pressed={active}
                            onClick={() => onChange(active ? value.filter((v) => v !== f.family) : [...value, f.family])}
                            className={chip(active)}
                        >
                            {f.label}
                            <span className="font-mono tabular-nums text-slate-400">{f.seeds}</span>
                        </button>
                    );
                })}
            </div>
        </div>
    );
}

export function InlineError({ message, compact = false }: { message: string; compact?: boolean }) {
    return (
        <p className={cn("flex items-start gap-1.5 text-[11.5px] leading-snug text-rose-600", !compact && "mt-1.5")}>
            <AlertCircleIcon className="w-3.5 h-3.5 shrink-0 mt-px" />
            <span>{message}</span>
        </p>
    );
}

export function CampaignPicker({ value, name, onChange }: { value: string; name?: string; onChange: (id: string) => void }) {
    const [open, setOpen] = React.useState(false);
    const [q, setQ] = React.useState("");
    const debounced = useDebouncedValue(q.trim(), 250);
    const list = useCampaigns({ query: debounced, folder: "", limit: 20, enabled: open, all: false });
    return (
        <PopoverMenu open={open} onOpenChange={setOpen}>
            <PopoverMenuTrigger asChild>
                <SelectButton
                    icon={<MegaphoneIcon className="w-3.5 h-3.5" />}
                    label={value ? (name ?? "Loading…") : "Pick a campaign"}
                    className="w-full [&>span:nth-child(2)]:max-w-none [&>span:nth-child(2)]:flex-1 [&>span:nth-child(2)]:text-left"
                />
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={280} className="p-1 max-h-80">
                <div className="p-1.5">
                    <SearchInput value={q} onChange={setQ} placeholder="Search campaigns…" autoFocus />
                </div>
                {list.isLoading && list.campaigns.length === 0 ? (
                    <div className="px-3 py-2 text-[11.5px] text-slate-400 inline-flex items-center gap-1.5">
                        <Loader2Icon className="w-3 h-3 animate-spin" /> Loading…
                    </div>
                ) : list.campaigns.length === 0 ? (
                    <div className="px-3 py-2 text-[11.5px] text-slate-400">No campaign matches that.</div>
                ) : (
                    list.campaigns.map((c) => (
                        <PopoverMenuItem key={c.id} selected={c.id === value} onSelect={() => onChange(c.id)}>
                            {c.name}
                        </PopoverMenuItem>
                    ))
                )}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

// Whose merge fields fill the copy. Empty = the campaign's first lead, or the
// built-in sample contact for custom copy.
export function ContactPicker({
    campaignId,
    value,
    onChange,
}: {
    campaignId: string;
    value: Contact | null;
    onChange: (c: Contact | null) => void;
}) {
    const [open, setOpen] = React.useState(false);
    const [q, setQ] = React.useState("");
    const debounced = useDebouncedValue(q.trim(), 250);
    const searching = debounced.length > 0;
    const search = useSearchContacts({
        options: {
            query: debounced,
            custom_field_filters: [],
            campaign_ids: searching || !campaignId ? [] : [campaignId],
            sort_by: "updated_at",
            reverse: false,
        },
        limit: 8,
        enabled: open,
        keepPrevious: true,
    });
    const contacts = search.contacts ?? [];
    const fallback = campaignId ? "The campaign's first lead" : "A sample contact";
    return (
        <PopoverMenu open={open} onOpenChange={setOpen}>
            <PopoverMenuTrigger asChild>
                <SelectButton
                    icon={<UserRoundIcon className="w-3.5 h-3.5" />}
                    label={value ? contactLabel(value) : fallback}
                    className="w-full [&>span:nth-child(2)]:max-w-none [&>span:nth-child(2)]:flex-1 [&>span:nth-child(2)]:text-left"
                />
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={300} matchTriggerWidth className="p-1">
                <div className="p-1.5">
                    <SearchInput value={q} onChange={setQ} placeholder="Search contacts…" autoFocus />
                </div>
                <PopoverMenuItem selected={value === null} onSelect={() => onChange(null)} icon={<UserRoundIcon className="w-3.5 h-3.5" />}>
                    {fallback}
                </PopoverMenuItem>
                <PopoverMenuSeparator />
                <PopoverMenuLabel>{searching || !campaignId ? "Contacts" : "Leads in this campaign"}</PopoverMenuLabel>
                <div className="max-h-56 overflow-y-auto">
                    {search.isLoading && contacts.length === 0 ? (
                        <div className="px-3 py-2 text-[11.5px] text-slate-400 inline-flex items-center gap-1.5">
                            <Loader2Icon className="w-3 h-3 animate-spin" /> Loading…
                        </div>
                    ) : contacts.length === 0 ? (
                        <div className="px-3 py-2 text-[11.5px] text-slate-400">
                            {searching ? "No contact matches that." : "No contacts yet. Type to search."}
                        </div>
                    ) : (
                        contacts.map((c) => (
                            <PopoverMenuItem key={c.id} selected={value?.id === c.id} onSelect={() => onChange(c)}>
                                <span className="text-slate-800">{contactLabel(c)}</span>
                                <span className="ml-1.5 text-[11px] text-slate-400">{c.email}</span>
                            </PopoverMenuItem>
                        ))
                    )}
                </div>
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
