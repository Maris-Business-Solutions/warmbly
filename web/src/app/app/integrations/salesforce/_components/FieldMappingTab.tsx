// Field mapping: which Warmbly field moves to or from which Salesforce field,
// per object, in which direction, and whether it may overwrite a value.

import React from "react";
import { AlertTriangleIcon, PlusIcon, RotateCcwIcon, Trash2Icon, XIcon } from "lucide-react";

import { TextInput } from "@/components/ui/field";
import { SelectMenu } from "@/components/ui/select-menu";
import { Segmented } from "@/components/app/campaigns/preferences/components/CampaignPreferenceBoolBox";
import { useConfirm } from "@/hooks/context/confirm";
import useCustomFieldKeys from "@/lib/api/hooks/app/contacts/useCustomFieldKeys";
import { useSalesforceMetadata } from "@/lib/api/hooks/app/integrations/useSalesforce";
import {
    SALESFORCE_DIRECTION_LABELS,
    SALESFORCE_POLICY_LABELS,
    type SalesforceFieldDirection,
    type SalesforceFieldInfo,
    type SalesforceFieldMapRow,
    type SalesforceFieldPolicy,
    type SalesforceObject,
    type SalesforceSettings,
    type SalesforceWarmblyField,
} from "@/lib/api/models/app/integrations/Salesforce";
import { cn } from "@/lib/utils";

import { SearchSelect, secondaryBtn, type SearchOption } from "./shared";
import { errMsg } from "./util";

type Patch = (fn: (s: SalesforceSettings) => SalesforceSettings) => void;

const CUSTOM_NEW = "__custom_new__";
const GRID = "sm:grid sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_170px_140px_28px] sm:items-center gap-2";

const isEngagement = (k: string) => k.startsWith("engagement.");
const isRelated = (f: string) => f.includes(".");

export default function FieldMappingTab({
    connectionId,
    draft,
    patch,
    warmblyFields,
    defaultFieldMap,
}: {
    connectionId: string;
    draft: SalesforceSettings;
    patch: Patch;
    warmblyFields: SalesforceWarmblyField[];
    defaultFieldMap: SalesforceFieldMapRow[];
}) {
    const confirm = useConfirm();
    const meta = useSalesforceMetadata(connectionId);
    const customKeys = useCustomFieldKeys();
    const [object, setObject] = React.useState<SalesforceObject>("Lead");

    const sfFields = React.useMemo(
        () => (object === "Lead" ? (meta.data?.lead_fields ?? []) : (meta.data?.contact_fields ?? [])),
        [meta.data, object],
    );

    const rows = draft.field_map
        .map((r, index) => ({ r, index }))
        .filter(({ r }) => r.object === object);
    const counts = {
        Lead: draft.field_map.filter((r) => r.object === "Lead").length,
        Contact: draft.field_map.filter((r) => r.object === "Contact").length,
    };

    // Two rules writing the same Salesforce field the same way are refused on save.
    const dupes = React.useMemo(() => {
        const seen = new Map<string, number>();
        for (const r of draft.field_map) {
            const k = `${r.object}|${r.salesforce.toLowerCase()}|${r.direction}`;
            seen.set(k, (seen.get(k) ?? 0) + 1);
        }
        return seen;
    }, [draft.field_map]);

    const warmblyOptions: SearchOption[] = React.useMemo(() => {
        const opts: SearchOption[] = warmblyFields.map((f) => ({
            value: f.key,
            label: f.label,
            hint: isEngagement(f.key) ? "Push only" : undefined,
        }));
        for (const k of customKeys.data ?? []) {
            opts.push({ value: `custom:${k}`, label: `Custom: ${k}`, hint: "Contact custom field" });
        }
        opts.push({ value: CUSTOM_NEW, label: "Custom field…", hint: "Type a custom field key" });
        return opts;
    }, [warmblyFields, customKeys.data]);

    function setRow(index: number, p: Partial<SalesforceFieldMapRow>) {
        patch((s) => ({
            ...s,
            field_map: s.field_map.map((r, i) => (i === index ? normalize({ ...r, ...p }) : r)),
        }));
    }
    function removeRow(index: number) {
        patch((s) => ({ ...s, field_map: s.field_map.filter((_, i) => i !== index) }));
    }
    function addRow() {
        patch((s) => ({
            ...s,
            field_map: [...s.field_map, { object, warmbly: "", salesforce: "", direction: "push", policy: "if_empty" }],
        }));
    }
    function resetDefaults() {
        confirm.show(
            "Replace every field rule, for Leads and Contacts, with the defaults? Nothing changes until you save.",
            async () => patch((s) => ({ ...s, field_map: defaultFieldMap.map((r) => ({ ...r })) })),
        );
    }

    return (
        <div className="space-y-4 max-w-5xl">
            <div className="flex flex-wrap items-center gap-3">
                <Segmented
                    value={object}
                    onChange={setObject}
                    options={[
                        { value: "Lead", label: `Lead · ${counts.Lead}` },
                        { value: "Contact", label: `Contact · ${counts.Contact}` },
                    ]}
                />
                <p className="text-[11.5px] text-slate-500 flex-1 min-w-[220px]">
                    Email always matches records. These rules move everything else.
                </p>
                <button type="button" onClick={resetDefaults} className={secondaryBtn}>
                    <RotateCcwIcon className="w-3 h-3" />
                    Reset to defaults
                </button>
            </div>

            {meta.isError && (
                <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 flex items-start gap-2 text-[11.5px] text-amber-800">
                    <AlertTriangleIcon className="w-3.5 h-3.5 mt-0.5 shrink-0" />
                    <span>
                        Could not read your Salesforce fields ({errMsg(meta.error, "unknown error")}). Type API names
                        instead.
                    </span>
                </div>
            )}

            <div className="rounded-md border border-slate-200 bg-white">
                <div className={cn("hidden px-3 h-8 border-b border-slate-200 bg-slate-50/60", GRID)}>
                    <HeadCell>Warmbly field</HeadCell>
                    <HeadCell>Salesforce field</HeadCell>
                    <HeadCell>Direction</HeadCell>
                    <HeadCell>When a value exists</HeadCell>
                    <span />
                </div>
                {rows.length === 0 ? (
                    <div className="px-4 py-8 text-center text-[12px] text-slate-400">
                        No {object} fields are mapped. Only email is matched.
                    </div>
                ) : (
                    <div className="divide-y divide-slate-100">
                        {rows.map(({ r, index }) => (
                            <FieldRow
                                key={index}
                                row={r}
                                sfFields={sfFields}
                                metaFailed={meta.isError}
                                metaLoading={meta.isPending}
                                warmblyOptions={warmblyOptions}
                                warmblyFields={warmblyFields}
                                duplicate={(dupes.get(`${r.object}|${r.salesforce.toLowerCase()}|${r.direction}`) ?? 0) > 1}
                                onChange={(p) => setRow(index, p)}
                                onRemove={() => removeRow(index)}
                            />
                        ))}
                    </div>
                )}
                <div className="px-3 py-2 border-t border-slate-100">
                    <button
                        type="button"
                        onClick={addRow}
                        disabled={draft.field_map.length >= 100}
                        className="h-7 px-2 rounded-md text-[12px] text-sky-700 hover:bg-sky-50 inline-flex items-center gap-1.5 transition-colors disabled:opacity-50"
                    >
                        <PlusIcon className="w-3.5 h-3.5" />
                        Add {object} field
                    </button>
                </div>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">
                Engagement fields only push. Related fields such as Account.Name can only be read. Two-way fields
                resolve conflicts with the policy: “Only fill blanks” never overwrites a value on either side.
            </p>
        </div>
    );
}

// Directions a rule may take for its two fields.
function allowedDirections(warmbly: string, salesforce: string, field?: SalesforceFieldInfo): SalesforceFieldDirection[] {
    if (isEngagement(warmbly)) return ["push"];
    if (isRelated(salesforce)) return ["pull"];
    if (field && (!field.updateable || field.calculated)) return ["pull"];
    return ["push", "pull", "both"];
}

function normalize(r: SalesforceFieldMapRow): SalesforceFieldMapRow {
    if (isEngagement(r.warmbly) && r.direction !== "push") return { ...r, direction: "push" };
    if (isRelated(r.salesforce) && r.direction !== "pull") return { ...r, direction: "pull" };
    return r;
}

function FieldRow({
    row,
    sfFields,
    metaFailed,
    metaLoading,
    warmblyOptions,
    warmblyFields,
    duplicate,
    onChange,
    onRemove,
}: {
    row: SalesforceFieldMapRow;
    sfFields: SalesforceFieldInfo[];
    metaFailed: boolean;
    metaLoading: boolean;
    warmblyOptions: SearchOption[];
    warmblyFields: SalesforceWarmblyField[];
    duplicate: boolean;
    onChange: (p: Partial<SalesforceFieldMapRow>) => void;
    onRemove: () => void;
}) {
    const field = sfFields.find((f) => f.name === row.salesforce);
    const directions = allowedDirections(row.warmbly, row.salesforce, field);
    const isCustom = row.warmbly.startsWith("custom:");
    const customKey = isCustom ? row.warmbly.slice("custom:".length) : "";
    // Writing needs an updateable field, so push and two-way rules only offer those.
    const writes = row.direction !== "pull";

    const sfOptions: SearchOption[] = sfFields.map((f) => {
        const readOnly = !f.updateable || f.calculated || isRelated(f.name);
        return {
            value: f.name,
            label: f.label,
            hint: f.name,
            disabled: writes && readOnly,
            disabledReason: `${f.name} · read-only, switch the rule to Salesforce → Warmbly`,
        };
    });

    const problem = !row.warmbly || (isCustom && !customKey.trim())
        ? "Choose a Warmbly field"
        : !row.salesforce
          ? "Choose a Salesforce field"
          : duplicate
            ? `${row.salesforce} is mapped twice in this direction`
            : null;

    return (
        <div className={cn("px-3 py-2.5 space-y-2 sm:space-y-0", GRID)}>
            <div className="min-w-0">
                <MobileLabel>Warmbly field</MobileLabel>
                {isCustom ? (
                    <div className="flex items-center gap-1">
                        <span className="h-7 px-1.5 rounded-md bg-slate-100 text-[10.5px] text-slate-500 inline-flex items-center shrink-0">
                            custom
                        </span>
                        <TextInput
                            value={customKey}
                            onChange={(v) => onChange({ warmbly: `custom:${v.replace(/\s+/g, "_")}` })}
                            placeholder="field_key"
                            className="flex-1 font-mono text-[12px]"
                            invalid={!customKey.trim()}
                        />
                        <button
                            type="button"
                            onClick={() => onChange({ warmbly: "" })}
                            aria-label="Choose a standard field"
                            className="size-7 rounded-md text-slate-400 hover:text-slate-700 hover:bg-slate-100 inline-flex items-center justify-center shrink-0"
                        >
                            <XIcon className="w-3 h-3" />
                        </button>
                    </div>
                ) : (
                    <SearchSelect
                        value={row.warmbly}
                        valueLabel={warmblyFields.find((f) => f.key === row.warmbly)?.label}
                        onChange={(v) => onChange({ warmbly: v === CUSTOM_NEW ? "custom:" : v })}
                        options={warmblyOptions}
                        placeholder="Choose a field"
                        searchPlaceholder="Search Warmbly fields…"
                        className="w-full"
                        aria-label="Warmbly field"
                    />
                )}
            </div>
            <div className="min-w-0">
                <MobileLabel>Salesforce field</MobileLabel>
                {metaFailed ? (
                    <TextInput
                        value={row.salesforce}
                        onChange={(v) => onChange({ salesforce: v.replace(/[^A-Za-z0-9_.]/g, "") })}
                        placeholder="API name, e.g. Title"
                        className="w-full font-mono text-[12px]"
                    />
                ) : (
                    <SearchSelect
                        value={row.salesforce}
                        valueLabel={field?.label ?? row.salesforce}
                        onChange={(v) => onChange({ salesforce: v })}
                        options={sfOptions}
                        loading={metaLoading}
                        placeholder="Choose a field"
                        searchPlaceholder="Search Salesforce fields…"
                        className="w-full"
                        minWidth={320}
                        aria-label="Salesforce field"
                    />
                )}
            </div>
            <div className="min-w-0">
                <MobileLabel>Direction</MobileLabel>
                <SelectMenu
                    value={row.direction}
                    onChange={(v) => onChange({ direction: v as SalesforceFieldDirection })}
                    fullWidth
                    aria-label="Direction"
                    options={(["push", "pull", "both"] as SalesforceFieldDirection[]).map((d) => ({
                        value: d,
                        label: SALESFORCE_DIRECTION_LABELS[d],
                        disabled: !directions.includes(d),
                    }))}
                />
            </div>
            <div className="min-w-0">
                <MobileLabel>When a value exists</MobileLabel>
                <SelectMenu
                    value={row.policy}
                    onChange={(v) => onChange({ policy: v as SalesforceFieldPolicy })}
                    fullWidth
                    aria-label="Overwrite policy"
                    options={(["if_empty", "overwrite"] as SalesforceFieldPolicy[]).map((p) => ({
                        value: p,
                        label: SALESFORCE_POLICY_LABELS[p],
                    }))}
                />
            </div>
            <div className="flex sm:justify-end">
                <button
                    type="button"
                    onClick={onRemove}
                    aria-label="Remove field rule"
                    className="size-7 rounded-md text-slate-400 hover:text-rose-600 hover:bg-rose-50 inline-flex items-center justify-center transition-colors"
                >
                    <Trash2Icon className="w-3.5 h-3.5" />
                </button>
            </div>
            {problem && (
                <p className="sm:col-span-5 text-[11px] text-amber-700 inline-flex items-center gap-1">
                    <AlertTriangleIcon className="w-3 h-3" />
                    {problem}
                </p>
            )}
        </div>
    );
}

function HeadCell({ children }: { children: React.ReactNode }) {
    return <span className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{children}</span>;
}

function MobileLabel({ children }: { children: React.ReactNode }) {
    return <div className="sm:hidden text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium mb-1">{children}</div>;
}
