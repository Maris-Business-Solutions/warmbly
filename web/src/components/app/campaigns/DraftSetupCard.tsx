// What a draft still needs before it can launch: contacts, an email, senders.
// Each row links to the tab that fixes it, "Continue setup" reopens the draft
// in the new-campaign flow, and launch opens the page's launch dialog.

import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { CheckIcon, MailIcon, PencilLineIcon, RocketIcon, SendIcon, UsersIcon } from "lucide-react";
import type Campaign from "@/lib/api/models/app/campaigns/Campaign";
import getSequences from "@/lib/api/client/app/campaigns/sequences/getSequences";
import { usePermission } from "@/hooks/usePermission";
import { cn } from "@/lib/utils";
import { NewCampaignDialog } from "./NewCampaignDialog";

export default function DraftSetupCard({ campaign, leads }: { campaign: Campaign; leads: number | undefined }) {
    const canSend = usePermission("SEND_CAMPAIGNS");
    const canEdit = usePermission("MANAGE_CAMPAIGNS");
    const [setupOpen, setSetupOpen] = useState(false);
    const steps = useQuery({
        queryKey: ["campaigns", campaign.id, "sequences"],
        queryFn: () => getSequences(campaign.id),
        // Only a draft shows the card.
        enabled: campaign.status === "draft",
    });
    if (campaign.status !== "draft") return null;

    const emails = (steps.data ?? []).filter((s) => (s.kind ?? "email") === "email").length;
    const explicit = campaign.sender_strategy === "explicit";
    const senderCount = explicit ? (campaign.senders?.length ?? 0) : (campaign.email_tags?.length ?? 0);
    const base = `/app/campaigns/${campaign.id}`;
    const rows = [
        {
            icon: UsersIcon,
            done: (leads ?? 0) > 0,
            title: "Contacts",
            value: leads === undefined ? "…" : leads > 0 ? `${leads.toLocaleString()} lead${leads === 1 ? "" : "s"}` : "No leads yet",
            to: `${base}/leads`,
            action: "Add contacts",
        },
        {
            icon: MailIcon,
            done: emails > 0,
            title: "Emails",
            value: steps.isPending ? "…" : emails > 0 ? `${emails} email${emails === 1 ? "" : "s"} written` : "Nothing written yet",
            to: `${base}/steps`,
            action: "Write",
        },
        {
            icon: SendIcon,
            done: true,
            title: "Senders",
            value:
                senderCount === 0
                    ? "Every active mailbox"
                    : explicit
                      ? `${senderCount} chosen mailbox${senderCount === 1 ? "" : "es"}`
                      : `${senderCount} mailbox tag${senderCount === 1 ? "" : "s"}`,
            to: `${base}/preferences`,
            action: "Change",
        },
    ];
    const ready = rows.every((r) => r.done);

    return (
        <div className="rounded-md border border-slate-200 bg-white overflow-hidden">
            <div className="px-4 pt-3.5 pb-2 flex items-start gap-3">
                <div className="min-w-0">
                    <p className="text-[13px] font-semibold text-slate-900">{ready ? "Ready to launch" : "Finish setting up"}</p>
                    <p className="text-[11.5px] text-slate-500 mt-0.5">
                        {ready
                            ? "Launching runs the pre-send checks first, so nothing goes out until you confirm."
                            : "This campaign is a draft. Nothing sends until it launches."}
                    </p>
                </div>
                <div className="ml-auto shrink-0 flex items-center gap-1.5">
                    {canEdit && (
                        <button
                            type="button"
                            onClick={() => setSetupOpen(true)}
                            className={cn(
                                "h-7 px-3 rounded-md text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors",
                                ready ? "border border-slate-200 bg-white hover:bg-slate-50 text-slate-700" : "bg-sky-600 hover:bg-sky-700 text-white",
                            )}
                        >
                            <PencilLineIcon className="w-3 h-3" />
                            Continue setup
                        </button>
                    )}
                    {canSend && ready && (
                        <Link
                            to={`${base}?launch=1`}
                            className="h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors"
                        >
                            <RocketIcon className="w-3 h-3" />
                            Launch
                        </Link>
                    )}
                </div>
            </div>
            <div className="grid sm:grid-cols-3 gap-px bg-slate-100 border-t border-slate-100">
                {rows.map((r) => (
                    <Link key={r.title} to={r.to} className="group bg-white px-4 py-3 flex items-center gap-3 hover:bg-slate-50 transition-colors">
                        <span
                            className={cn(
                                "size-6 rounded-full inline-flex items-center justify-center shrink-0",
                                r.done ? "bg-sky-600 text-white" : "bg-white text-slate-400 ring-1 ring-inset ring-slate-200",
                            )}
                        >
                            {r.done ? <CheckIcon className="w-3 h-3" strokeWidth={3} /> : <r.icon className="w-3 h-3" />}
                        </span>
                        <span className="min-w-0 flex-1">
                            <span className="block text-[11px] text-slate-500">{r.title}</span>
                            <span className={cn("block text-[12.5px] truncate", r.done ? "text-slate-900" : "text-slate-600")}>{r.value}</span>
                        </span>
                        <span
                            className={cn(
                                "shrink-0 text-[11.5px] transition-opacity",
                                r.done ? "text-slate-400 opacity-100 md:opacity-0 md:group-hover:opacity-100" : "text-sky-700",
                            )}
                        >
                            {r.action}
                        </span>
                    </Link>
                ))}
            </div>
            <NewCampaignDialog open={setupOpen} draftId={setupOpen ? campaign.id : null} onClose={() => setSetupOpen(false)} />
        </div>
    );
}
