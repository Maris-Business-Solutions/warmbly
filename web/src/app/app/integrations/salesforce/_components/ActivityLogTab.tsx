// Activity log: every event Warmbly queued for Salesforce, its outcome, the
// Salesforce error when it failed, and retry for failed or skipped ones.

import React from "react";
import {
    CalendarCheckIcon,
    CheckIcon,
    ExternalLinkIcon,
    Loader2Icon,
    MailIcon,
    MailOpenIcon,
    MailWarningIcon,
    MousePointerClickIcon,
    ReplyIcon,
    RotateCwIcon,
    UserXIcon,
    XIcon,
    type LucideIcon,
} from "lucide-react";
import toast from "react-hot-toast/headless";

import { Checkbox } from "@/components/ui/checkbox";
import ScrollStrip from "@/components/ui/scroll-strip";
import ContactEdit from "@/components/app/contacts/ContactEdit";
import useContact from "@/lib/api/hooks/app/contacts/useContact";
import {
    useRetrySalesforceActivity,
    useSalesforceActivity,
} from "@/lib/api/hooks/app/integrations/useSalesforce";
import {
    SALESFORCE_ACTIVITY_LABELS,
    salesforceRecordURL,
    type SalesforceActivity,
    type SalesforceActivityKind,
    type SalesforceActivityStatus,
} from "@/lib/api/models/app/integrations/Salesforce";
import { cn } from "@/lib/utils";

import { Pill, secondaryBtn } from "./shared";
import { absolute, ago, errMsg } from "./util";

const FILTERS: { value: SalesforceActivityStatus | ""; label: string }[] = [
    { value: "", label: "All" },
    { value: "failed", label: "Failed" },
    { value: "pending", label: "Pending" },
    { value: "synced", label: "Synced" },
    { value: "skipped", label: "Skipped" },
];

const KIND_ICON: Record<SalesforceActivityKind, LucideIcon> = {
    sent: MailIcon,
    replied: ReplyIcon,
    opened: MailOpenIcon,
    clicked: MousePointerClickIcon,
    bounced: MailWarningIcon,
    unsubscribed: UserXIcon,
    meeting_booked: CalendarCheckIcon,
};

const STATUS_TONE: Record<SalesforceActivityStatus, "emerald" | "sky" | "slate" | "rose"> = {
    synced: "emerald",
    pending: "sky",
    skipped: "slate",
    failed: "rose",
};

const retryable = (a: SalesforceActivity) => a.status === "failed" || a.status === "skipped";

export default function ActivityLogTab({
    connectionId,
    instanceUrl,
    status: rawStatus,
    onStatus,
}: {
    connectionId: string;
    instanceUrl: string;
    status: string;
    onStatus: (s: string) => void;
}) {
    const status = (FILTERS.some((f) => f.value === rawStatus) ? rawStatus : "") as SalesforceActivityStatus | "";
    const activity = useSalesforceActivity(connectionId, status);
    const retry = useRetrySalesforceActivity(connectionId);
    const [selected, setSelected] = React.useState<Set<string>>(new Set());
    const [openContact, setOpenContact] = React.useState("");

    // A selection never outlives the filter it was made under.
    React.useEffect(() => setSelected(new Set()), [status]);

    const rows = activity.rows;
    const selectable = rows.filter(retryable);
    const allSelected = selectable.length > 0 && selectable.every((r) => selected.has(r.id));

    function toggle(id: string) {
        setSelected((s) => {
            const n = new Set(s);
            if (n.has(id)) n.delete(id);
            else n.add(id);
            return n;
        });
    }
    function toggleAll() {
        setSelected(allSelected ? new Set() : new Set(selectable.map((r) => r.id)));
    }

    function runRetry(ids?: string[]) {
        retry.mutate(
            { connectionId, ids },
            {
                onSuccess: (res) => {
                    toast.success(
                        res.requeued === 0
                            ? "Nothing to retry"
                            : `Queued ${res.requeued.toLocaleString()} ${res.requeued === 1 ? "event" : "events"} again`,
                    );
                    setSelected(new Set());
                },
                onError: (err) => toast.error(errMsg(err, "Could not retry")),
            },
        );
    }

    return (
        <div className="space-y-3 max-w-6xl">
            <div className="flex flex-wrap items-center gap-2">
                <ScrollStrip activeKey={status || "all"} className="flex-1 min-w-0" innerClassName="gap-1">
                    {FILTERS.map((f) => {
                        const active = f.value === status;
                        return (
                            <button
                                key={f.label}
                                type="button"
                                data-active={active}
                                onClick={() => onStatus(f.value)}
                                className={cn(
                                    "h-7 px-2.5 rounded-md text-[12px] whitespace-nowrap shrink-0 transition-colors",
                                    active
                                        ? "bg-sky-50 text-sky-700 font-medium"
                                        : "text-slate-500 hover:text-slate-900 hover:bg-slate-100",
                                )}
                            >
                                {f.label}
                            </button>
                        );
                    })}
                </ScrollStrip>
                <button
                    type="button"
                    onClick={() => runRetry()}
                    disabled={retry.isPending}
                    className={secondaryBtn}
                    title="Queues every failed event again"
                >
                    {retry.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <RotateCwIcon className="w-3 h-3" />}
                    Retry all failed
                </button>
            </div>

            <div className="rounded-md border border-slate-200 bg-white overflow-x-auto">
                <table className="w-full text-[12px] min-w-[860px]">
                    <thead>
                        <tr className="border-b border-slate-200 bg-slate-50/60 text-left">
                            <th className="w-9 pl-3">
                                <Checkbox
                                    checked={allSelected}
                                    onChange={toggleAll}
                                    disabled={selectable.length === 0}
                                    aria-label="Select all retryable events"
                                />
                            </th>
                            {["Event", "Contact", "Status", "Detail", "Tries", "When", ""].map((h, i) => (
                                <th key={i} className="px-3 h-8 text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">
                                    {h}
                                </th>
                            ))}
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100">
                        {activity.isPending ? (
                            <tr>
                                <td colSpan={8} className="px-3 py-10 text-center text-slate-400">
                                    <Loader2Icon className="w-3.5 h-3.5 animate-spin inline mr-1.5" />
                                    Loading activity…
                                </td>
                            </tr>
                        ) : activity.isError ? (
                            <tr>
                                <td colSpan={8} className="px-3 py-8 text-center text-rose-700">
                                    {errMsg(activity.error, "Could not load the activity log")}
                                </td>
                            </tr>
                        ) : rows.length === 0 ? (
                            <tr>
                                <td colSpan={8} className="px-3 py-10 text-center text-slate-400">
                                    {status ? `No ${status} events.` : "Nothing has been sent to Salesforce yet."}
                                </td>
                            </tr>
                        ) : (
                            rows.map((a) => (
                                <ActivityRow
                                    key={a.id}
                                    a={a}
                                    instanceUrl={instanceUrl}
                                    selected={selected.has(a.id)}
                                    onToggle={() => toggle(a.id)}
                                    onOpenContact={() => a.contact_id && setOpenContact(a.contact_id)}
                                />
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {activity.hasNextPage && (
                <div className="flex justify-center">
                    <button
                        type="button"
                        onClick={() => void activity.fetchNextPage()}
                        disabled={activity.isFetchingNextPage}
                        className={secondaryBtn}
                    >
                        {activity.isFetchingNextPage && <Loader2Icon className="w-3 h-3 animate-spin" />}
                        Load more
                    </button>
                </div>
            )}

            {selected.size > 0 && (
                <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-30 flex items-center max-w-[calc(100vw-16px)] gap-1.5 rounded-md border border-slate-200 bg-white shadow-[0_6px_20px_-4px_rgba(15,23,42,0.12),0_2px_4px_rgba(15,23,42,0.04)] px-2 py-1.5">
                    <div className="inline-flex items-center gap-1.5 px-2 h-7 rounded bg-sky-50 text-sky-700 text-[12px] font-medium">
                        <CheckIcon className="w-3 h-3" />
                        {selected.size.toLocaleString()} selected
                    </div>
                    <button
                        type="button"
                        onClick={() => runRetry([...selected])}
                        disabled={retry.isPending}
                        className="h-7 px-2.5 rounded text-[12px] text-slate-700 hover:text-slate-900 hover:bg-slate-100 font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                    >
                        {retry.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <RotateCwIcon className="w-3 h-3" />}
                        Retry
                    </button>
                    <button
                        type="button"
                        onClick={() => setSelected(new Set())}
                        aria-label="Clear selection"
                        className="size-7 rounded text-slate-400 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center justify-center transition-colors"
                    >
                        <XIcon className="w-3.5 h-3.5" />
                    </button>
                </div>
            )}

            {openContact && <ContactQuickView contactId={openContact} onClose={() => setOpenContact("")} />}
        </div>
    );
}

function ActivityRow({
    a,
    instanceUrl,
    selected,
    onToggle,
    onOpenContact,
}: {
    a: SalesforceActivity;
    instanceUrl: string;
    selected: boolean;
    onToggle: () => void;
    onOpenContact: () => void;
}) {
    const Icon = KIND_ICON[a.kind] ?? MailIcon;
    const taskUrl = salesforceRecordURL(instanceUrl, a.task_id);
    const recordUrl = salesforceRecordURL(instanceUrl, a.record_id);
    return (
        <tr className={cn("align-top", selected && "bg-sky-50/40")}>
            <td className="pl-3 py-2">
                {retryable(a) && (
                    <Checkbox checked={selected} onChange={onToggle} aria-label={`Select ${a.contact_email}`} />
                )}
            </td>
            <td className="px-3 py-2 whitespace-nowrap">
                <span className="inline-flex items-center gap-1.5 text-slate-800">
                    <Icon className="w-3.5 h-3.5 text-slate-400" />
                    {SALESFORCE_ACTIVITY_LABELS[a.kind] ?? a.kind}
                </span>
            </td>
            <td className="px-3 py-2 max-w-[220px]">
                {a.contact_id ? (
                    <button
                        type="button"
                        onClick={onOpenContact}
                        className="text-sky-700 hover:underline truncate max-w-full text-left"
                    >
                        {a.contact_email}
                    </button>
                ) : (
                    <span className="text-slate-700 truncate block">{a.contact_email}</span>
                )}
            </td>
            <td className="px-3 py-2">
                <Pill tone={STATUS_TONE[a.status] ?? "slate"}>{a.status}</Pill>
            </td>
            <td className="px-3 py-2 max-w-[320px]">
                {a.detail ? (
                    <span
                        className={cn(
                            "break-words text-[11.5px]",
                            a.status === "failed" ? "text-rose-700 font-mono" : "text-slate-500",
                        )}
                    >
                        {a.detail}
                    </span>
                ) : (
                    <span className="text-slate-300">None</span>
                )}
                {a.status === "pending" && a.attempts > 0 && (
                    <div className="text-[10.5px] text-slate-400 mt-0.5">Next try {absolute(a.next_attempt_at)}</div>
                )}
            </td>
            <td className="px-3 py-2 tabular-nums text-slate-600">{a.attempts}</td>
            <td className="px-3 py-2 whitespace-nowrap text-slate-500" title={absolute(a.occurred_at)}>
                {ago(a.occurred_at)}
            </td>
            <td className="px-3 py-2 whitespace-nowrap">
                <span className="inline-flex items-center gap-2">
                    {taskUrl && (
                        <a
                            href={taskUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-[11.5px] text-sky-700 hover:underline inline-flex items-center gap-1"
                        >
                            Task
                            <ExternalLinkIcon className="w-2.5 h-2.5" />
                        </a>
                    )}
                    {recordUrl && (
                        <a
                            href={recordUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-[11.5px] text-sky-700 hover:underline inline-flex items-center gap-1"
                        >
                            Record
                            <ExternalLinkIcon className="w-2.5 h-2.5" />
                        </a>
                    )}
                </span>
            </td>
        </tr>
    );
}

// Opens the contact drawer for one id without leaving the page.
function ContactQuickView({ contactId, onClose }: { contactId: string; onClose: () => void }) {
    const detail = useContact(contactId);
    const contacts = React.useMemo(() => (detail.data ? [detail.data] : []), [detail.data]);

    React.useEffect(() => {
        if (detail.isError) {
            toast.error(errMsg(detail.error, "Could not open the contact"));
            onClose();
        }
    }, [detail.isError, detail.error, onClose]);

    return (
        <ContactEdit
            contacts={contacts}
            active={contactId}
            setActive={(v) => {
                const next = typeof v === "function" ? v(contactId) : v;
                if (!next) onClose();
            }}
        />
    );
}
