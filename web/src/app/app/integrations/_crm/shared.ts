import type { ExternalCrm } from "@/components/app/crm/crmProviders";
import type {
    CRMFieldDirection,
    CRMOption,
    CRMProviderConfig,
} from "@/lib/api/models/app/crm/CRMProvider";

export type ConfigPatch = (fn: (c: CRMProviderConfig) => CRMProviderConfig) => void;

export interface PickerOption {
    value: string;
    label: string;
    hint?: string;
}

// The server's defaults, used until settings load and to fill gaps in old rows.
const DEFAULT_CONFIGS: Record<ExternalCrm, CRMProviderConfig> = {
    hubspot: {
        activity: { sent: true, replies: true, bounces: true, unsubscribes: true, meetings: true, opens: false, clicks: false },
        create_contacts: true,
        create_companies: true,
        write_properties: true,
        positive_reply: { lead_status: "IN_PROGRESS", lifecycle_stage: "", create_deal: false },
        exit_rules: { deal_created: true, lifecycle_stages: ["opportunity", "customer"], opted_out: true },
        guards: { skip_lifecycle_stages: ["customer", "evangelist"], skip_open_deals: true, skip_other_owners: false, skip_opted_out: true },
        deal_pipelines: [],
        display_properties: [],
        field_map: { first_name: "firstname", last_name: "lastname", company: "company", phone: "phone" },
        field_direction: {},
    },
    // Pipedrive's label ids are only known once connected; the server seeds
    // them from the company's own labels at the switch.
    pipedrive: {
        activity: { sent: true, replies: true, bounces: true, unsubscribes: true, meetings: true, opens: false, clicks: false },
        create_contacts: true,
        create_companies: true,
        write_properties: true,
        positive_reply: { lead_status: "", lifecycle_stage: "", create_deal: false, create_lead: false },
        exit_rules: { deal_created: true, lifecycle_stages: [], opted_out: true },
        guards: { skip_lifecycle_stages: [], skip_open_deals: true, skip_other_owners: false, skip_opted_out: true },
        deal_pipelines: [],
        display_properties: [],
        field_map: { first_name: "first_name", last_name: "last_name", company: "org_name", phone: "phone" },
        field_direction: {},
    },
};

export function defaultConfig(provider: ExternalCrm): CRMProviderConfig {
    return structuredClone(DEFAULT_CONFIGS[provider]);
}

// The stored choices when they were made for this provider, else its defaults.
export function configFor(provider: ExternalCrm, c?: Partial<CRMProviderConfig> | null): CRMProviderConfig {
    const owner = c?.for || "hubspot";
    return normalizeConfig(owner === provider ? c : null, provider);
}

// Go marshals empty slices and maps as null; the editors want real values.
export function normalizeConfig(c?: Partial<CRMProviderConfig> | null, provider: ExternalCrm = "hubspot"): CRMProviderConfig {
    const d = DEFAULT_CONFIGS[provider];
    if (!c) return structuredClone(d);
    return {
        activity: { ...d.activity, ...(c.activity ?? {}) },
        create_contacts: c.create_contacts ?? d.create_contacts,
        create_companies: c.create_companies ?? d.create_companies,
        write_properties: c.write_properties ?? d.write_properties,
        positive_reply: { ...d.positive_reply, ...(c.positive_reply ?? {}) },
        exit_rules: {
            ...d.exit_rules,
            ...(c.exit_rules ?? {}),
            lifecycle_stages: c.exit_rules?.lifecycle_stages ?? [],
        },
        guards: {
            ...d.guards,
            ...(c.guards ?? {}),
            skip_lifecycle_stages: c.guards?.skip_lifecycle_stages ?? [],
        },
        deal_pipelines: c.deal_pipelines ?? [],
        display_properties: c.display_properties ?? [],
        field_map: c.field_map ?? {},
        field_direction: c.field_direction ?? {},
        for: c.for,
    };
}

export function sameConfig(a: unknown, b: unknown): boolean {
    return JSON.stringify(a) === JSON.stringify(b);
}

const BASE_FIELDS: Record<string, string> = {
    first_name: "First name",
    last_name: "Last name",
    email: "Email",
    company: "Company",
    phone: "Phone",
};

// The Warmbly contact fields a CRM property can map to (email is the match key).
export const MAPPABLE_FIELDS = ["first_name", "last_name", "company", "phone"];

export function warmblyFieldLabel(key: string): string {
    if (key.startsWith("custom:")) return key.slice("custom:".length);
    return BASE_FIELDS[key] ?? key;
}

export function directionOptions(name: string): { value: CRMFieldDirection; label: string; hint: string }[] {
    return [
        { value: "both", label: "Most recent", hint: "Two-way. The latest edit on either side wins." },
        { value: "push", label: "Warmbly wins", hint: `Warmbly writes to ${name} and ignores ${name} edits.` },
        { value: "pull", label: `${name} wins`, hint: `${name} writes to Warmbly and Warmbly never overwrites it.` },
    ];
}

export function humanize(value: string): string {
    if (!value) return "";
    const s = value.replace(/[_-]+/g, " ").toLowerCase();
    return s.charAt(0).toUpperCase() + s.slice(1);
}

export function optionLabel(options: CRMOption[] | undefined, value: string): string {
    return options?.find((o) => o.value === value)?.label ?? humanize(value);
}

// Lifecycle stage and label pickers keep stored values the CRM no longer
// lists, so nothing disappears silently.
export function withStoredValues(options: CRMOption[] | undefined, stored: string[]): PickerOption[] {
    const out: PickerOption[] = (options ?? []).map((o) => ({ value: o.value, label: o.label }));
    for (const v of stored) if (!out.some((o) => o.value === v)) out.push({ value: v, label: humanize(v) });
    return out;
}

export function timeAgo(at?: Date | string | null): string {
    if (!at) return "never";
    const d = typeof at === "string" ? new Date(at) : at;
    const s = Math.max(0, Math.round((Date.now() - d.getTime()) / 1000));
    if (s < 45) return "just now";
    const m = Math.round(s / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.round(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.round(h / 24)}d ago`;
}

export function errMessage(err: unknown, fallback = "Something went wrong"): string {
    const e = err as { message?: string; error?: string } | null;
    return e?.message || e?.error || fallback;
}

export function errCode(err: unknown): string | undefined {
    return (err as { code?: string } | null)?.code;
}

export function plural(n: number, one: string, many = `${one}s`): string {
    return `${n.toLocaleString()} ${n === 1 ? one : many}`;
}

// "12 deals, 30 tasks and 8 notes"
export function joinList(parts: string[]): string {
    if (parts.length <= 1) return parts.join("");
    return `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}`;
}

export function replyOutcomeIssue(config: CRMProviderConfig): string | null {
    const r = config.positive_reply;
    if (r.create_deal && (!r.deal_pipeline_id || !r.deal_stage_id)) {
        return "Pick a pipeline and stage for new deals, or turn off Create a deal.";
    }
    return null;
}
