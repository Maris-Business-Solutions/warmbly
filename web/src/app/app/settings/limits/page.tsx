// Customer-facing limit-increase request flow. Lists past requests
// and offers a form to submit a new one. Approval surfaces as the
// effective limit going up on the org; rejection surfaces with the
// admin's notes attached to the row.

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";
import { Link } from "@tanstack/react-router";
import { SelectMenu, type SelectOption } from "@/components/ui/select-menu";
import { NumberInput } from "@/components/ui/field";
import { Section, SectionShell } from "../_components/SectionShell";
import getCurrentOrganization from "@/lib/api/client/app/organizations/getCurrentOrganization";
import listLimitRequests from "@/lib/api/client/app/organizations/listLimitRequests";
import submitLimitRequest from "@/lib/api/client/app/organizations/submitLimitRequest";
import cancelLimitRequest from "@/lib/api/client/app/organizations/cancelLimitRequest";
import useBrand from "@/hooks/useBrand";
import useFeatureAccess from "@/hooks/useFeatureAccess";
import useOrganizationLimits from "@/lib/api/hooks/app/organizations/useOrganizationLimits";
import type OrganizationLimits from "@/lib/api/models/app/organizations/OrganizationLimits";
import type {
    LimitField,
    LimitRequestStatus,
} from "@/lib/api/models/app/organizations/LimitIncreaseRequest";

const FIELD_OPTIONS: { value: LimitField; label: string; hint: string }[] = [
    { value: "max_email_accounts", label: "Mailboxes", hint: "More connected sending mailboxes" },
    { value: "max_campaigns", label: "Campaigns (lifetime)", hint: "Higher cap on total campaigns created" },
    { value: "max_active_campaigns", label: "Active campaigns", hint: "More campaigns running at the same time" },
    { value: "max_team_members", label: "Team members", hint: "More seats on this workspace" },
    { value: "max_contacts", label: "Contacts", hint: "Store more recipient records" },
    { value: "daily_campaign_limit", label: "Daily sends", hint: "Send more campaign emails per day" },
];

// What the workspace uses of each limit, read beside the limit itself.
function usageFor(field: LimitField, data: OrganizationLimits): number {
    const c = data.counts;
    switch (field) {
        case "max_email_accounts":
            return data.mailboxes?.used ?? c.email_accounts;
        case "max_campaigns":
            return c.total_campaigns;
        case "max_active_campaigns":
            return c.active_campaigns;
        case "max_team_members":
            return c.total_members;
        case "max_contacts":
            return c.total_contacts;
        case "daily_campaign_limit":
            return c.emails_sent_today;
    }
}

// Null is unmetered. Only the mailbox allowance can be; the server refuses a request for it.
function limitFor(field: LimitField, data: OrganizationLimits): number | null {
    if (field === "max_email_accounts") return data.mailboxes ? data.mailboxes.allowance : (data.limits.max_email_accounts ?? null);
    return data.limits[field] ?? null;
}

const STATUS_TONE: Record<LimitRequestStatus, string> = {
    pending: "bg-amber-50 text-amber-700 border-amber-200",
    approved: "bg-emerald-50 text-emerald-700 border-emerald-200",
    rejected: "bg-red-50 text-red-700 border-red-200",
    cancelled: "bg-slate-50 text-slate-600 border-slate-200",
};

export default function LimitsSettingsPage() {
    const qc = useQueryClient();
    const brand = useBrand();

    const orgQuery = useQuery({
        queryKey: ["app", "organizations", "current"],
        queryFn: getCurrentOrganization,
    });
    const orgId = orgQuery.data?.id;

    const requestsQuery = useQuery({
        queryKey: ["app", "organizations", orgId, "limit-requests"],
        queryFn: () => listLimitRequests(orgId!),
        enabled: !!orgId,
    });

    const limitsQuery = useOrganizationLimits();
    const limits = limitsQuery.data;
    // Free and the Warmup plan do not send, so only the mailbox allowance applies; the server refuses the rest.
    const sends = !useFeatureAccess().locked;

    // An unmetered resource has nothing to raise, so it is not offered.
    const unlimited = useMemo(
        () => (limits ? FIELD_OPTIONS.filter((o) => limitFor(o.value, limits) === null) : []),
        [limits],
    );
    const requestable = useMemo(
        () =>
            FIELD_OPTIONS.filter(
                (o) => !unlimited.some((u) => u.value === o.value) && (sends || o.value === "max_email_accounts"),
            ),
        [unlimited, sends],
    );

    const [chosen, setChosen] = useState<LimitField | null>(null);
    const field: LimitField | undefined =
        requestable.find((o) => o.value === chosen)?.value ?? requestable[0]?.value;
    const [requested, setRequested] = useState<number>(Number.NaN);
    const [reason, setReason] = useState<string>("");

    const fieldSelectOptions = useMemo<SelectOption[]>(
        () => requestable.map((opt) => ({ value: opt.value, label: opt.label })),
        [requestable],
    );

    const currentLimit = field && limits ? limitFor(field, limits) : null;
    const currentUsage = field && limits ? usageFor(field, limits) : null;

    const submit = useMutation({
        mutationFn: () =>
            submitLimitRequest(orgId!, {
                field: field!,
                requested,
                reason,
            }),
        onSuccess: () => {
            toast.success("Request submitted — an admin will review shortly.");
            qc.invalidateQueries({ queryKey: ["app", "organizations", orgId, "limit-requests"] });
            qc.invalidateQueries({ queryKey: ["organizations", "limits"] });
            setRequested(Number.NaN);
            setReason("");
        },
        onError: (err: Error) => {
            toast.error(err.message || "Could not submit — please try again.");
        },
    });

    const cancel = useMutation({
        mutationFn: (id: string) => cancelLimitRequest(id),
        onSuccess: () => {
            toast.success("Request cancelled");
            qc.invalidateQueries({ queryKey: ["app", "organizations", orgId, "limit-requests"] });
        },
        onError: (err: Error) => {
            toast.error(err.message || "Cancel failed");
        },
    });

    const rows = requestsQuery.data?.data ?? [];
    const pending = rows.find((r) => r.status === "pending" && r.field === field);

    function onSubmit(e: React.FormEvent) {
        e.preventDefault();
        if (!field || pending) return;
        const n = requested;
        if (!Number.isInteger(n) || n <= 0) {
            toast.error("Requested value must be a positive integer");
            return;
        }
        if (currentLimit !== null && n <= currentLimit) {
            toast.error(`Ask for more than your current ${currentLimit.toLocaleString()}`);
            return;
        }
        if (reason.trim().length < 10) {
            toast.error("Please include a reason (at least a sentence)");
            return;
        }
        submit.mutate();
    }

    return (
        <SectionShell
            title="Limits"
            description="Ask for more capacity than your plan or our product-level cap allows. Increases are reviewed and may be refused per our terms of service."
        >
            <Section
                eyebrow="Request an increase"
                description="Tell us what you need and why. We aim to respond within one business day."
            >
                {unlimited.length > 0 && (
                    <p className="mb-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-[12px] text-emerald-800">
                        {unlimited.map((o) => o.label).join(", ")} are unlimited on your plan, so there is nothing to
                        request.
                        {unlimited.some((o) => o.value === "max_email_accounts") && limits?.mailboxes && (
                            <>
                                {" "}
                                All {limits.mailboxes.used.toLocaleString()} connected mailbox
                                {limits.mailboxes.used === 1 ? " is" : "es are"} on the{" "}
                                <Link to="/app/emails" className="font-medium underline hover:text-emerald-950">
                                    Accounts page
                                </Link>
                                .
                            </>
                        )}
                    </p>
                )}
                {!sends && (
                    <p className="mb-3 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-[12px] text-slate-600">
                        This workspace warms mailboxes and does not send, so sending, contact, campaign and seat limits do
                        not apply yet. They come with a{" "}
                        <Link to="/app/settings/billing/{-$tab}" params={{ tab: undefined }} className="font-medium underline hover:text-slate-900">
                            plan that sends
                        </Link>
                        .
                    </p>
                )}
                {limitsQuery.isPending ? (
                    <p className="text-[12px] text-slate-500">Loading…</p>
                ) : field && (
                <form onSubmit={onSubmit} className="space-y-3">
                    <div>
                        <label className="text-[12px] font-medium text-slate-700">Resource</label>
                        <SelectMenu
                            value={field}
                            onChange={(v) => {
                                setChosen(v as LimitField);
                                setRequested(Number.NaN);
                            }}
                            options={fieldSelectOptions}
                            className="mt-1 w-full"
                            aria-label="Resource"
                        />
                        <p className="text-[11px] text-slate-500 mt-1">
                            {FIELD_OPTIONS.find((o) => o.value === field)?.hint}
                            {currentLimit !== null && currentUsage !== null && (
                                <>
                                    {" · "}
                                    Using {currentUsage.toLocaleString()} of {currentLimit.toLocaleString()}
                                </>
                            )}
                        </p>
                    </div>
                    {pending ? (
                        <div className="flex items-center gap-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-[12px] text-amber-800">
                            <span className="min-w-0">
                                You already asked for {pending.requested.toLocaleString()}, and it is waiting for review.
                            </span>
                            <button
                                type="button"
                                onClick={() => cancel.mutate(pending.id)}
                                disabled={cancel.isPending}
                                className="ml-auto shrink-0 text-[11px] font-medium underline hover:text-amber-950 disabled:opacity-50"
                            >
                                Withdraw it
                            </button>
                        </div>
                    ) : (
                    <>
                    <div>
                        <label className="text-[12px] font-medium text-slate-700">
                            Requested value
                        </label>
                        <NumberInput
                            min={currentLimit !== null ? currentLimit + 1 : 1}
                            value={requested}
                            onChange={setRequested}
                            className="mt-1 flex w-full"
                            placeholder={currentLimit !== null ? `More than ${currentLimit.toLocaleString()}` : "e.g. 50"}
                        />
                    </div>
                    <div>
                        <label className="text-[12px] font-medium text-slate-700">Reason</label>
                        <textarea
                            value={reason}
                            onChange={(e) => setReason(e.target.value)}
                            rows={3}
                            className="mt-1 block w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm"
                            placeholder="Why does this matter for your team? Volume, customer commitments, ramp plans, etc."
                        />
                    </div>
                    <div className="flex items-center gap-3">
                        <button
                            type="submit"
                            disabled={submit.isPending || !orgId}
                            className="rounded-md bg-slate-900 px-3 py-2 text-sm font-medium text-white hover:bg-slate-800 disabled:opacity-50"
                        >
                            {submit.isPending ? "Submitting…" : "Submit request"}
                        </button>
                        <p className="text-[11px] text-slate-500">
                            {brand.terms_url ? (
                                <>
                                    Subject to review per our{" "}
                                    <a href={brand.terms_url} target="_blank" rel="noreferrer" className="underline">
                                        terms of service
                                    </a>
                                    .
                                </>
                            ) : (
                                "Subject to review."
                            )}
                        </p>
                    </div>
                    </>
                    )}
                </form>
                )}
            </Section>

            <Section eyebrow="Your requests" description="Pending, approved, and historical decisions.">
                {requestsQuery.isLoading ? (
                    <p className="text-[12px] text-slate-500">Loading…</p>
                ) : rows.length === 0 ? (
                    <p className="text-[12px] text-slate-500">No requests yet.</p>
                ) : (
                    <ul className="space-y-2">
                        {rows.map((r) => {
                            const fieldLabel =
                                FIELD_OPTIONS.find((o) => o.value === r.field)?.label ?? r.field;
                            return (
                                <li
                                    key={r.id}
                                    className="rounded-md border border-slate-200 p-3 bg-white"
                                >
                                    <div className="flex items-start justify-between gap-3">
                                        <div className="min-w-0">
                                            <div className="text-sm font-medium">
                                                {fieldLabel}: {r.current_effective.toLocaleString()}
                                                {" → "}
                                                {r.requested.toLocaleString()}
                                            </div>
                                            <div className="text-[11px] text-slate-500 mt-1 break-words">
                                                {new Date(r.submitted_at).toLocaleDateString()} · "{r.reason}"
                                            </div>
                                            {r.review_notes && r.status !== "pending" && (
                                                <div className="text-[11px] text-slate-600 mt-1 italic break-words">
                                                    Reviewer: "{r.review_notes}"
                                                </div>
                                            )}
                                        </div>
                                        <div className="flex flex-col items-end gap-1.5 shrink-0">
                                            <span
                                                className={`text-[10px] px-1.5 py-0.5 rounded border ${STATUS_TONE[r.status]}`}
                                            >
                                                {r.status}
                                            </span>
                                            {r.status === "pending" && (
                                                <button
                                                    onClick={() => cancel.mutate(r.id)}
                                                    disabled={cancel.isPending}
                                                    className="text-[11px] text-slate-500 hover:text-slate-800 underline"
                                                >
                                                    Cancel
                                                </button>
                                            )}
                                        </div>
                                    </div>
                                </li>
                            );
                        })}
                    </ul>
                )}
            </Section>
        </SectionShell>
    );
}
