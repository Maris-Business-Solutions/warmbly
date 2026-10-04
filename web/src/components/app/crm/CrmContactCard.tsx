// The connected CRM's side of one contact: owner, lifecycle stage (a label in
// Pipedrive) and lead status, edited in place and written to the CRM. Shared by
// the inbox panel and the contact drawer so both read and edit it the same way.

import React from "react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
    AlertTriangleIcon,
    BuildingIcon,
    ChevronDownIcon,
    ExternalLinkIcon,
    Loader2Icon,
    PlusIcon,
    RefreshCwIcon,
} from "lucide-react";
import { PopoverMenu, PopoverMenuContent, PopoverMenuItem, PopoverMenuTrigger } from "@/components/ui/popover-menu";
import { SearchInput } from "@/components/ui/field";
import { usePermission } from "@/hooks/usePermission";
import useCrmProvider from "@/hooks/useCrmProvider";
import { useCrmContact, useLinkCrmContact, useUpdateCrmContact } from "@/lib/api/hooks/app/crm/provider/useCrmContact";
import useCrmMetadata from "@/lib/api/hooks/app/crm/provider/useCrmMetadata";
import useCrmOwners from "@/lib/api/hooks/app/crm/provider/useCrmOwners";
import getCrmContact from "@/lib/api/client/app/crm/provider/getCrmContact";
import type { CRMOption, CRMOwner, CRMPropertyView, UpdateCRMContact } from "@/lib/api/models/app/crm/CRMProvider";
import type { AppError } from "@/lib/api/client/normalizeError";
import { cn } from "@/lib/utils";
import { type CrmInfo, CrmBadge, CrmSyncedAt, OpenInCrm } from "./crmProviders";
import { crmErrorMessage } from "./crmModeUtils";

type Field = "owner" | "lifecycle" | "lead";

interface FieldError {
    field: Field | "link";
    message: string;
    reauth: boolean;
}

function toFieldError(field: FieldError["field"], err: unknown, crm: CrmInfo): FieldError {
    const e = err as AppError;
    if (e?.code === "crm_contact_missing") {
        return {
            field,
            reauth: false,
            message:
                e.message ||
                `${crm.name} has no ${crm.words.contact} with this email address, and creating them from Warmbly is turned off.`,
        };
    }
    return { field, message: crmErrorMessage(err), reauth: e?.code === "crm_reauth_required" };
}

function ownerName(o?: Pick<CRMOwner, "first_name" | "last_name" | "email">): string {
    if (!o) return "";
    return `${o.first_name ?? ""} ${o.last_name ?? ""}`.trim() || o.email || "";
}

// Reads the cached CRM view without asking for another pull; the card on the
// same contact already did.
function useCrmContactView(contactId: string | undefined, enabled = true) {
    return useQuery({
        queryKey: ["crm", "contact", contactId],
        queryFn: () => getCrmContact(contactId as string),
        enabled: enabled && !!contactId,
        staleTime: 30_000,
    });
}

export default function CrmContactCard({
    contactId,
    density = "panel",
    showProperties = true,
    className,
}: {
    contactId: string;
    // "panel" for the narrow inbox rail, "drawer" for the contact slide-over.
    density?: "panel" | "drawer";
    showProperties?: boolean;
    className?: string;
}) {
    const { isExternal, crm, needsReconnect } = useCrmProvider();
    const view = useCrmContact(contactId, isExternal);
    const canEdit = usePermission("MANAGE_CONTACTS");
    const link = useLinkCrmContact();
    const update = useUpdateCrmContact();
    const linked = !!view.data?.linked;
    const metadata = useCrmMetadata(isExternal && linked);
    const owners = useCrmOwners(isExternal && linked && canEdit);

    // The value being written, so the row shows the choice before the CRM answers.
    const [pending, setPending] = React.useState<{ field: Field; label: string } | null>(null);
    const [error, setError] = React.useState<FieldError | null>(null);

    // A different contact starts clean.
    React.useEffect(() => {
        setPending(null);
        setError(null);
    }, [contactId]);

    if (!isExternal) return null;

    const data = view.data;
    const drawer = density === "drawer";

    async function save(field: Field, body: UpdateCRMContact, label: string) {
        setPending({ field, label });
        setError(null);
        try {
            await update.mutateAsync({ contactId, data: body });
        } catch (err) {
            setError(toFieldError(field, err, crm));
        } finally {
            setPending(null);
        }
    }

    async function addToCrm() {
        setError(null);
        try {
            await link.mutateAsync(contactId);
        } catch (err) {
            setError(toFieldError("link", err, crm));
        }
    }

    const ownerOptions: PickerOption[] = (owners.data ?? [])
        .filter((o) => !o.archived || o.external_id === data?.owner?.external_id)
        .map((o) => ({ value: o.external_id, label: ownerName(o) || o.external_id, hint: o.email }))
        .sort((a, b) => a.label.localeCompare(b.label));
    const lifecycleOptions = optionList(metadata.data?.lifecycle_stages, data?.lifecycle_stage);
    const leadOptions = optionList(metadata.data?.lead_statuses, data?.lead_status);

    return (
        <div className={cn("min-w-0", className)}>
            <div className="flex items-center gap-2 mb-2 min-h-6">
                <CrmBadge provider={crm.id} />
                {data?.linked && data.url && (
                    <OpenInCrm url={data.url} provider={crm.id} compact={!drawer} className="ml-auto" />
                )}
                {view.isFetching && !view.isPending && (
                    <Loader2Icon
                        className={cn("w-3 h-3 animate-spin text-slate-300", data?.linked && data.url ? "" : "ml-auto")}
                        aria-label={`Syncing with ${crm.name}`}
                    />
                )}
            </div>

            {needsReconnect && (
                <Link
                    to={crm.settingsPath}
                    className="mb-2 flex items-start gap-1.5 rounded-md border border-amber-200 bg-amber-50/70 px-2 py-1.5 text-[11px] leading-snug text-amber-800 hover:bg-amber-50 transition-colors"
                >
                    <AlertTriangleIcon className="w-3 h-3 mt-px shrink-0" />
                    <span>Reconnect {crm.name} to keep this contact in sync.</span>
                </Link>
            )}

            {view.isPending ? (
                <div className="space-y-1">
                    {[0, 1, 2].map((i) => (
                        <div key={i} className="h-7 rounded-md bg-slate-100 animate-pulse" />
                    ))}
                </div>
            ) : view.isError && !data ? (
                <div className="rounded-md border border-slate-200 px-2.5 py-2 flex items-center gap-2">
                    <span className="flex-1 min-w-0 text-[11.5px] text-slate-600">
                        {crmErrorMessage(view.error)}
                    </span>
                    <button
                        type="button"
                        onClick={() => void view.refetch()}
                        className="shrink-0 h-6 px-2 rounded-md border border-slate-200 hover:border-slate-300 text-[11px] text-slate-700 inline-flex items-center gap-1 transition-colors"
                    >
                        <RefreshCwIcon className="w-3 h-3" />
                        Retry
                    </button>
                </div>
            ) : !data?.linked ? (
                <div className="rounded-md border border-dashed border-slate-200 px-2.5 py-2.5">
                    <p className="text-[12px] font-medium text-slate-700">Not in {crm.name} yet</p>
                    <p className="text-[11px] text-slate-500 leading-snug mt-0.5">
                        Add them to {crm.name} to see their owner, {crm.words.lifecycle.toLowerCase()} and deals here.
                    </p>
                    {canEdit && (
                        <button
                            type="button"
                            onClick={addToCrm}
                            disabled={link.isPending}
                            className={cn(
                                "mt-2 h-7 px-2.5 rounded-md border text-[11.5px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60",
                                crm.border,
                                crm.tint,
                                crm.tintText,
                            )}
                        >
                            {link.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <PlusIcon className="w-3 h-3" />}
                            Add to {crm.name}
                        </button>
                    )}
                    {error?.field === "link" && <InlineError error={error} />}
                </div>
            ) : (
                <>
                    {data.opted_out && (
                        <div className="mb-2 flex items-start gap-1.5 rounded-md border border-red-200 bg-red-50/60 px-2 py-1.5">
                            <AlertTriangleIcon className="w-3 h-3 text-red-600 mt-px shrink-0" />
                            <span className="text-[11px] text-red-700 leading-snug">
                                {crm.id === "pipedrive" ? "Unsubscribed from email in Pipedrive." : "Opted out of email in HubSpot."}
                            </span>
                        </div>
                    )}
                    <div className="rounded-md border border-slate-200 bg-white">
                        <Row label="Owner" drawer={drawer}>
                            <Picker
                                label={pending?.field === "owner" ? pending.label : ownerName(data.owner)}
                                empty="No owner"
                                options={ownerOptions}
                                value={data.owner?.external_id}
                                loading={owners.isPending}
                                saving={pending?.field === "owner"}
                                disabled={!canEdit}
                                searchable
                                onPick={(o) => void save("owner", { owner_external_id: o.value }, o.label)}
                            />
                        </Row>
                        {error?.field === "owner" && <InlineError error={error} inRow />}
                        <Row label={crm.words.lifecycle} drawer={drawer}>
                            <Picker
                                label={pending?.field === "lifecycle" ? pending.label : data.lifecycle_stage?.label ?? ""}
                                empty="Not set"
                                options={lifecycleOptions}
                                value={data.lifecycle_stage?.value}
                                loading={metadata.isPending}
                                saving={pending?.field === "lifecycle"}
                                disabled={!canEdit}
                                onPick={(o) => void save("lifecycle", { lifecycle_stage: o.value }, o.label)}
                            />
                        </Row>
                        {error?.field === "lifecycle" && <InlineError error={error} inRow />}
                        {crm.words.leadStatus && (
                            <Row label={crm.words.leadStatus} drawer={drawer}>
                                <Picker
                                    label={pending?.field === "lead" ? pending.label : data.lead_status?.label ?? ""}
                                    empty="Not set"
                                    options={leadOptions}
                                    value={data.lead_status?.value}
                                    loading={metadata.isPending}
                                    saving={pending?.field === "lead"}
                                    disabled={!canEdit}
                                    onPick={(o) => void save("lead", { lead_status: o.value }, o.label)}
                                />
                            </Row>
                        )}
                        {error?.field === "lead" && <InlineError error={error} inRow />}
                        {data.company && (
                            <Row label={crm.words.company === "company" ? "Company" : "Organization"} drawer={drawer}>
                                <CompanyValue
                                    hover={crm.hoverText}
                                    name={data.company.name}
                                    domain={data.company.domain}
                                    url={data.company.url}
                                />
                            </Row>
                        )}
                        {showProperties &&
                            data.properties.map((p) => (
                                <Row key={p.name} label={p.label || p.name} drawer={drawer}>
                                    <PropertyValue value={p.value} />
                                </Row>
                            ))}
                    </div>
                    {data.synced_at && <CrmSyncedAt at={data.synced_at} provider={crm.id} className="mt-1.5" />}
                </>
            )}
        </div>
    );
}

// The displayed CRM properties as plain rows, for the drawer's Details tab.
export function CrmPropertiesList({ contactId }: { contactId: string }) {
    const { isExternal, crm } = useCrmProvider();
    const view = useCrmContactView(contactId, isExternal);
    if (!isExternal) return null;
    const data = view.data;
    const props: CRMPropertyView[] = data?.properties ?? [];

    if (view.isPending) {
        return <div className="h-16 rounded-md bg-slate-100 animate-pulse" />;
    }
    if (!data?.linked) {
        return <p className="text-[11.5px] text-slate-500">This contact is not in {crm.name} yet.</p>;
    }
    if (props.length === 0) {
        return (
            <p className="text-[11.5px] text-slate-500">
                No {crm.name} {crm.words.properties} are chosen for display.{" "}
                <Link to={crm.settingsPath} className="text-sky-700 hover:underline">
                    Pick them in {crm.name} settings
                </Link>
                .
            </p>
        );
    }
    return (
        <div>
            <div className="rounded-md border border-slate-200 bg-white">
                {props.map((p) => (
                    <Row key={p.name} label={p.label || p.name} drawer>
                        <PropertyValue value={p.value} />
                    </Row>
                ))}
            </div>
            <div className="mt-1.5 flex items-center gap-2">
                {data.synced_at && <CrmSyncedAt at={data.synced_at} provider={crm.id} />}
                {data.url && <OpenInCrm url={data.url} provider={crm.id} className="ml-auto" />}
            </div>
        </div>
    );
}

interface PickerOption {
    value: string;
    label: string;
    hint?: string;
}

function optionList(options: CRMOption[] | undefined, current?: CRMOption): PickerOption[] {
    const list = (options ?? []).map((o) => ({ value: o.value, label: o.label || o.value }));
    // A value the CRM no longer offers still shows as the current choice.
    if (current?.value && !list.some((o) => o.value === current.value)) {
        list.unshift({ value: current.value, label: current.label || current.value });
    }
    return list;
}

function Row({ label, drawer, children }: { label: string; drawer: boolean; children: React.ReactNode }) {
    return (
        <div
            className={cn(
                "flex items-center gap-2 border-b last:border-b-0 border-slate-100 min-w-0",
                drawer ? "px-3 py-1 min-h-8" : "px-2 py-0.5 min-h-7",
            )}
        >
            <div className={cn("shrink-0 text-slate-500 truncate", drawer ? "w-28 text-[11px]" : "w-[84px] text-[10.5px]")} title={label}>
                {label}
            </div>
            <div className="min-w-0 flex-1 flex justify-end">{children}</div>
        </div>
    );
}

function PropertyValue({ value }: { value: string }) {
    if (!value) return <span className="text-[12px] text-slate-300">Not set</span>;
    return (
        <span className="text-[12px] text-slate-900 text-right break-words min-w-0" title={value}>
            {value}
        </span>
    );
}

function CompanyValue({ name, domain, url, hover }: { name: string; domain?: string; url?: string; hover: string }) {
    const body = (
        <>
            <BuildingIcon className="w-3 h-3 text-slate-400 shrink-0" />
            <span className="truncate">{name || domain}</span>
            {url && <ExternalLinkIcon className="w-2.5 h-2.5 opacity-60 shrink-0" />}
        </>
    );
    if (!url) {
        return (
            <span className="inline-flex items-center gap-1 min-w-0 text-[12px] text-slate-900" title={domain}>
                {body}
            </span>
        );
    }
    return (
        <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            title={domain ? `${name} · ${domain}` : name}
            className={cn("inline-flex items-center gap-1 min-w-0 text-[12px] text-slate-900 transition-colors", hover)}
        >
            {body}
        </a>
    );
}

function Picker({
    label,
    empty,
    options,
    value,
    loading,
    saving,
    disabled,
    searchable,
    onPick,
}: {
    label: string;
    empty: string;
    options: PickerOption[];
    value?: string;
    loading: boolean;
    saving: boolean;
    disabled: boolean;
    searchable?: boolean;
    onPick: (o: PickerOption) => void;
}) {
    const [open, setOpen] = React.useState(false);
    const [q, setQ] = React.useState("");
    const { crm } = useCrmProvider();

    React.useEffect(() => {
        if (!open) setQ("");
    }, [open]);

    const text = label || empty;
    if (disabled) {
        return <span className={cn("text-[12px] truncate", label ? "text-slate-900" : "text-slate-400")}>{text}</span>;
    }

    const needle = q.trim().toLowerCase();
    const shown = needle
        ? options.filter((o) => o.label.toLowerCase().includes(needle) || o.hint?.toLowerCase().includes(needle))
        : options;

    return (
        <PopoverMenu open={open} onOpenChange={setOpen} align="end">
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    disabled={saving}
                    className="max-w-full h-6 pl-1.5 pr-1 rounded-md text-[12px] inline-flex items-center gap-1 min-w-0 hover:bg-slate-100 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sky-100 disabled:cursor-wait"
                >
                    <span className={cn("truncate", label ? "text-slate-900" : "text-slate-400")}>{text}</span>
                    {saving ? (
                        <Loader2Icon className="w-3 h-3 animate-spin text-slate-400 shrink-0" />
                    ) : (
                        <ChevronDownIcon className="w-3 h-3 text-slate-400 shrink-0" />
                    )}
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={200} className="max-h-72">
                {searchable && (
                    <div className="px-2 pt-1 pb-1.5 border-b border-slate-100 mb-1">
                        <SearchInput value={q} onChange={setQ} placeholder={`Search ${crm.words.owners}…`} autoFocus className="w-full" />
                    </div>
                )}
                {loading ? (
                    <div className="px-3 py-2 flex items-center gap-1.5 text-[11.5px] text-slate-400">
                        <Loader2Icon className="w-3 h-3 animate-spin" />
                        Loading from {crm.name}…
                    </div>
                ) : shown.length === 0 ? (
                    <div className="px-3 py-2 text-[11.5px] text-slate-400">{needle ? "No match" : "Nothing to choose"}</div>
                ) : (
                    shown.map((o) => (
                        <PopoverMenuItem
                            key={o.value}
                            selected={o.value === value}
                            onSelect={() => {
                                if (o.value !== value) onPick(o);
                            }}
                            trailing={
                                o.hint && o.hint !== o.label ? (
                                    <span className="text-[10.5px] text-slate-400 truncate max-w-[140px] inline-block align-middle">
                                        {o.hint}
                                    </span>
                                ) : undefined
                            }
                        >
                            {o.label}
                        </PopoverMenuItem>
                    ))
                )}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

function InlineError({ error, inRow }: { error: FieldError; inRow?: boolean }) {
    const { crm } = useCrmProvider();
    return (
        <div
            role="alert"
            className={cn(
                "flex items-start gap-1.5 text-[11px] leading-snug text-red-700",
                inRow ? "px-2 py-1.5 bg-red-50/60 border-b border-slate-100" : "mt-2",
            )}
        >
            <AlertTriangleIcon className="w-3 h-3 mt-px shrink-0" />
            <span className="min-w-0">
                {error.message}
                {error.reauth && (
                    <>
                        {" "}
                        <Link to={crm.settingsPath} className="underline hover:text-red-900">
                            Reconnect {crm.name}
                        </Link>
                    </>
                )}
            </span>
        </div>
    );
}
