// Admin outreach composer + audit log. Send platform email from
// noreply@warmbly.com with a configurable Reply-To so customer replies route
// to a real inbox. Every send is recorded in admin_outreach_messages; the log
// below the composer is a faceted, server-paged Explorer.

import { useEffect, useState } from "react";
import {
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";
import { Send } from "lucide-react";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Section, StatusBadge } from "@/components/ui/kit";
import {
    Explorer,
    FilterGroup,
    SearchFilter,
    SegmentedFilter,
    SelectFilter,
    ToggleFilter,
    DateRangeFilter,
} from "@/components/data/Explorer";
import { DataTable, type Column } from "@/components/data/DataTable";
import { useCursorPager } from "@/lib/useCursorPager";
import { TONE_TEXT, type Tone } from "@/lib/tones";
import {
    emptyRange,
    rangeActive,
    rangeWithin,
    rangeAfter,
    rangeBefore,
    type DateRange,
} from "@/lib/dateRange";
import { listOutreach, sendOutreach } from "@/lib/api/client/admin/outreach";
import type {
    AdminOutreachMessage,
    AdminOutreachSearch,
    AdminOutreachStatus,
    SendAdminOutreachRequest,
} from "@/lib/api/models/admin";

const STATUS_TONE: Record<AdminOutreachStatus, Tone> = {
    queued: "warning",
    sent: "success",
    failed: "danger",
};

const FIELD_LABEL = "mb-1.5 block text-xs font-medium text-muted-foreground";

type Mode = "user_id" | "org_id" | "email";

export default function OutreachPage() {
    const qc = useQueryClient();
    const [mode, setMode] = useState<Mode>("email");
    const [target, setTarget] = useState("");
    // Blank, not our support address: prefilling it on somebody else's
    // instance addresses their customers' replies to us.
    const [replyTo, setReplyTo] = useState("");
    const [subject, setSubject] = useState("");
    const [body, setBody] = useState("");

    // Log facets.
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState<"any" | AdminOutreachStatus>("any");
    const [recipientType, setRecipientType] = useState("");
    const [sentByQ, setSentByQ] = useState("");
    const [hasReplyTo, setHasReplyTo] = useState(false);
    const [hasError, setHasError] = useState(false);
    const [hasUser, setHasUser] = useState(false);
    const [hasOrg, setHasOrg] = useState(false);
    const [created, setCreated] = useState<DateRange>(emptyRange);
    const [sentAt, setSentAt] = useState<DateRange>(emptyRange);
    const [sort, setSort] = useState<{ by: string; desc: boolean }>({ by: "", desc: true });
    const pager = useCursorPager();
    const { reset } = pager;

    const filterKey = JSON.stringify({
        query, status, recipientType, sentByQ, hasReplyTo, hasError, hasUser, hasOrg, created, sentAt, sort,
    });

    useEffect(() => {
        reset();
    }, [filterKey, reset]);

    const { data, isLoading, error, refetch } = useQuery({
        queryKey: ["admin", "outreach", filterKey, pager.cursor],
        queryFn: () =>
            listOutreach({
                q: query.trim() || undefined,
                status: status === "any" ? undefined : status,
                recipient_type: (recipientType || undefined) as AdminOutreachSearch["recipient_type"],
                sent_by_q: sentByQ.trim() || undefined,
                has_reply_to: hasReplyTo || undefined,
                has_error: hasError || undefined,
                has_user: hasUser || undefined,
                has_org: hasOrg || undefined,
                created_within: rangeWithin(created),
                created_after: rangeAfter(created),
                created_before: rangeBefore(created),
                sent_at_after: rangeAfter(sentAt),
                sent_at_before: rangeBefore(sentAt),
                limit: 50,
                cursor: pager.cursor,
                sort_by: sort.by ? (sort.by as AdminOutreachSearch["sort_by"]) : undefined,
                sort_desc: sort.by ? sort.desc : undefined,
            }),
        staleTime: 30_000,
        refetchInterval: 30_000,
        placeholderData: keepPreviousData,
    });

    const send = useMutation({
        mutationFn: () => {
            const req: SendAdminOutreachRequest = { subject, body };
            if (replyTo.trim()) req.reply_to = replyTo.trim();
            if (mode === "email") req.to_email = target.trim();
            if (mode === "user_id") req.to_user_id = target.trim();
            if (mode === "org_id") req.to_org_id = target.trim();
            return sendOutreach(req);
        },
        onSuccess: () => {
            toast.success("Outreach sent");
            qc.invalidateQueries({ queryKey: ["admin", "outreach"] });
            setSubject("");
            setBody("");
            setTarget("");
        },
        onError: (err: Error) => toast.error(err.message || "Send failed"),
    });

    function submit(e: React.FormEvent) {
        e.preventDefault();
        if (!target.trim()) {
            toast.error("Recipient is required");
            return;
        }
        if (!subject.trim()) {
            toast.error("Subject is required");
            return;
        }
        if (!body.trim()) {
            toast.error("Body is required");
            return;
        }
        send.mutate();
    }

    const rows = data?.data ?? [];

    const bools = [hasReplyTo, hasError, hasUser, hasOrg];
    const activeCount =
        (query ? 1 : 0) +
        (status !== "any" ? 1 : 0) +
        (recipientType ? 1 : 0) +
        (sentByQ ? 1 : 0) +
        bools.filter(Boolean).length +
        [created, sentAt].filter(rangeActive).length +
        (sort.by ? 1 : 0);

    function resetAll() {
        setQuery("");
        setStatus("any");
        setRecipientType("");
        setSentByQ("");
        setHasReplyTo(false);
        setHasError(false);
        setHasUser(false);
        setHasOrg(false);
        setCreated(emptyRange);
        setSentAt(emptyRange);
        setSort({ by: "", desc: true });
    }

    const recipientLabel = (m: AdminOutreachMessage) => {
        if (m.to_user) return m.to_user.email;
        if (m.to_org_id) return "org owner";
        return "raw email";
    };

    const columns: Column<AdminOutreachMessage>[] = [
        {
            id: "when",
            header: "When",
            sortable: true,
            sortKey: "created_at",
            cell: (m) => (
                <span className="text-xs whitespace-nowrap text-muted-foreground tabular-nums">{new Date(m.created_at).toLocaleString()}</span>
            ),
            csv: (m) => m.created_at,
        },
        {
            id: "status",
            header: "Status",
            sortable: true,
            sortKey: "status",
            cell: (m) => (
                <div className="space-y-1 py-1">
                    <StatusBadge tone={STATUS_TONE[m.status]} dot className="capitalize">
                        {m.status}
                    </StatusBadge>
                    {m.error && (
                        <div className={`max-w-xs truncate text-xs ${TONE_TEXT.danger}`} title={m.error}>
                            {m.error}
                        </div>
                    )}
                </div>
            ),
            csv: (m) => m.status,
        },
        {
            id: "to",
            header: "To",
            sortable: true,
            sortKey: "to_email",
            cell: (m) => (
                <div className="min-w-0">
                    <div className="truncate font-mono text-xs text-foreground">{m.to_email}</div>
                    {m.reply_to && <div className="truncate text-xs text-muted-foreground">Reply-to: {m.reply_to}</div>}
                </div>
            ),
            csv: (m) => m.to_email,
        },
        {
            id: "recipient",
            header: "Recipient",
            cell: (m) => <span className="text-[13px] text-muted-foreground">{recipientLabel(m)}</span>,
            csv: (m) => recipientLabel(m),
        },
        {
            id: "subject",
            header: "Subject",
            sortable: true,
            sortKey: "subject",
            cell: (m) => (
                <span className="block max-w-md truncate text-[13px] text-foreground" title={m.subject}>
                    {m.subject}
                </span>
            ),
            csv: (m) => m.subject,
        },
        {
            id: "sender",
            header: "Sender",
            cell: (m) => <span className="text-xs text-muted-foreground">{m.sent_by_user?.email ?? m.sent_by.slice(0, 8)}</span>,
            csv: (m) => m.sent_by_user?.email ?? m.sent_by,
        },
        {
            id: "delivered",
            header: "Delivered",
            sortable: true,
            sortKey: "sent_at",
            defaultHidden: true,
            cell: (m) => (
                <span className="text-xs whitespace-nowrap text-muted-foreground tabular-nums">
                    {m.sent_at ? new Date(m.sent_at).toLocaleString() : "—"}
                </span>
            ),
            csv: (m) => m.sent_at ?? "",
        },
    ];

    return (
        <div>
            <PageHeader
                title="Outreach"
                description="Send platform email from the Warmbly noreply address with a configurable Reply-To. Every message is audit-logged below."
            />

            <Section title="Compose" description="Sent from the platform noreply address. Replies go to the Reply-To when one is set.">
                <form onSubmit={submit} className="max-w-4xl overflow-hidden surface-lit rounded-xl border border-border bg-card">
                    <div className="grid gap-4 p-4 md:grid-cols-2">
                        <div className="md:col-span-2">
                            <Label htmlFor="outreach-target" className={FIELD_LABEL}>
                                Recipient
                            </Label>
                            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                                <Select value={mode} onValueChange={(v) => setMode(v as Mode)}>
                                    <SelectTrigger aria-label="Recipient type" className="sm:w-56 sm:shrink-0">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="email">Email address</SelectItem>
                                        <SelectItem value="user_id">User ID</SelectItem>
                                        <SelectItem value="org_id">Org ID (sends to owner)</SelectItem>
                                    </SelectContent>
                                </Select>
                                <Input
                                    id="outreach-target"
                                    value={target}
                                    onChange={(e) => setTarget(e.target.value)}
                                    placeholder={mode === "email" ? "support-customer@example.com" : "uuid"}
                                    className="font-mono text-[12.5px]"
                                />
                            </div>
                        </div>

                        <div>
                            <Label htmlFor="reply_to" className={FIELD_LABEL}>
                                Reply-To
                            </Label>
                            <Input
                                id="reply_to"
                                value={replyTo}
                                onChange={(e) => setReplyTo(e.target.value)}
                                placeholder="support@yourdomain.com (replies will land here)"
                                className="font-mono text-[12.5px]"
                            />
                            <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">
                                From: defaults to the platform noreply address. Leave blank to
                                also have replies bounce against noreply.
                            </p>
                        </div>

                        <div>
                            <Label htmlFor="subject" className={FIELD_LABEL}>
                                Subject
                            </Label>
                            <Input
                                id="subject"
                                value={subject}
                                onChange={(e) => setSubject(e.target.value)}
                                placeholder="One-line subject"
                            />
                        </div>

                        <div className="md:col-span-2">
                            <Label htmlFor="body" className={FIELD_LABEL}>
                                Body (HTML)
                            </Label>
                            <Textarea
                                id="body"
                                value={body}
                                onChange={(e) => setBody(e.target.value)}
                                rows={10}
                                className="min-h-56 font-mono text-[12.5px]"
                                placeholder="<p>Hi…</p>"
                            />
                        </div>
                    </div>

                    <div className="flex items-center justify-end gap-2 border-t border-border bg-muted/40 px-4 py-2.5">
                        <Button type="submit" size="sm" disabled={send.isPending}>
                            <Send className="size-3.5" />
                            {send.isPending ? "Sending…" : "Send"}
                        </Button>
                    </div>
                </form>
            </Section>

            <Section title="Outreach log">
            <Explorer
                activeCount={activeCount}
                onReset={resetAll}
                filters={
                    <>
                        <FilterGroup label="Search">
                            <SearchFilter value={query} onChange={setQuery} placeholder="Email, subject, or reply-to…" />
                        </FilterGroup>
                        <FilterGroup label="Status">
                            <SegmentedFilter
                                value={status}
                                onChange={setStatus}
                                options={[
                                    { value: "any", label: "All" },
                                    { value: "queued", label: "Queued" },
                                    { value: "sent", label: "Sent" },
                                    { value: "failed", label: "Failed" },
                                ]}
                            />
                        </FilterGroup>
                        <FilterGroup label="Recipient type">
                            <SelectFilter
                                value={recipientType || "any"}
                                onChange={(v) => setRecipientType(v === "any" ? "" : v)}
                                options={[
                                    { value: "any", label: "Any recipient" },
                                    { value: "user", label: "User" },
                                    { value: "org", label: "Org owner" },
                                    { value: "email", label: "Raw email" },
                                ]}
                                placeholder="Any recipient"
                            />
                        </FilterGroup>
                        <FilterGroup label="Sender">
                            <SearchFilter value={sentByQ} onChange={setSentByQ} placeholder="Admin name or email…" />
                        </FilterGroup>
                        <FilterGroup label="Sent">
                            <DateRangeFilter value={created} onChange={setCreated} />
                        </FilterGroup>
                        <FilterGroup label="Delivered">
                            <DateRangeFilter value={sentAt} onChange={setSentAt} mode="custom" />
                        </FilterGroup>
                        <FilterGroup label="Flags">
                            <div className="flex flex-col gap-2">
                                <ToggleFilter checked={hasError} onChange={setHasError} label="Has error" />
                                <ToggleFilter checked={hasReplyTo} onChange={setHasReplyTo} label="Has reply-to" />
                                <ToggleFilter checked={hasUser} onChange={setHasUser} label="Linked to user" />
                                <ToggleFilter checked={hasOrg} onChange={setHasOrg} label="Linked to org" />
                            </div>
                        </FilterGroup>
                    </>
                }
            >
                <DataTable
                    columns={columns}
                    rows={rows}
                    getRowId={(m) => m.id}
                    loading={isLoading}
                    error={error}
                    onRetry={() => refetch()}
                    errorTitle="Failed to load outreach log"
                    sort={sort.by ? sort : undefined}
                    onSortChange={setSort}
                    storageKey="admin.outreach"
                    csvName="warmbly-outreach"
                    noun="messages"
                    emptyTitle="No outreach"
                    emptyHint="No messages match these filters."
                    pager={{
                        canPrev: pager.canPrev,
                        canNext: !!data?.pagination?.has_more,
                        onPrev: pager.prev,
                        onNext: () => pager.next(data?.pagination?.next_cursor),
                        page: pager.page,
                        shown: rows.length,
                        total: data?.pagination?.total ?? null,
                    }}
                />
            </Explorer>
            </Section>
        </div>
    );
}
