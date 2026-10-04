// CampaignFormsPanel — how the forms linked from this campaign performed for
// its recipients: links handed out, who opened them, who started, who
// submitted. Renders nothing when the campaign links to no forms.

import { Link } from "@tanstack/react-router";
import { ExternalLinkIcon } from "lucide-react";

import { SectionBar } from "@/components/layout/Page";
import AnimatedNumber from "@/components/ui/AnimatedNumber";
import { useCampaignForms } from "@/lib/api/hooks/app/forms";
import type CampaignFormStats from "@/lib/api/models/app/forms/CampaignFormStats";

function rate(numerator: number, denominator: number): string {
    if (denominator <= 0) return "–";
    return `${Math.min(100, (numerator / denominator) * 100).toFixed(1)}%`;
}

const STATUS_PILL: Record<CampaignFormStats["status"], string> = {
    draft: "bg-slate-100 text-slate-600",
    published: "bg-emerald-50 text-emerald-700",
    archived: "bg-amber-50 text-amber-700",
};

export default function CampaignFormsPanel({ campaignId }: { campaignId: string }) {
    const forms = useCampaignForms(campaignId);
    const rows = forms.data ?? [];

    // A campaign with no form in its steps should not grow an empty section.
    if (forms.isPending || rows.length === 0) return null;

    return (
        <div className="rounded-md border border-slate-200 overflow-hidden bg-white">
            <SectionBar label="Forms" count={rows.length} />
            <div className="divide-y divide-slate-200/60">
                <div className="h-8 px-5 flex items-center gap-3 text-[10px] uppercase tracking-[0.12em] text-slate-400 font-medium">
                    <span className="flex-1 min-w-0">Form</span>
                    <span className="w-16 text-right" title="Recipients given a personalized link">
                        Links
                    </span>
                    <span className="w-16 text-right">Opened</span>
                    <span className="w-16 text-right hidden md:block">Started</span>
                    <span className="w-16 text-right">Filled</span>
                    <span className="w-16 text-right hidden md:block" title="Submissions out of links sent">
                        Rate
                    </span>
                    <span className="w-7" />
                </div>
                {rows.map((f) => (
                    <div key={f.form_id} className="min-h-11 py-1.5 px-5 flex items-center gap-3">
                        <span className="flex items-center gap-2 flex-1 min-w-0">
                            <Link
                                to="/app/forms/$id"
                                params={{ id: f.form_id }}
                                className="text-[12.5px] text-slate-900 truncate hover:text-sky-700 transition-colors"
                            >
                                {f.form_name}
                            </Link>
                            {f.status !== "published" && (
                                <span
                                    className={`inline-flex items-center h-4 px-1.5 rounded text-[10px] font-medium shrink-0 ${STATUS_PILL[f.status]}`}
                                >
                                    {f.status}
                                </span>
                            )}
                        </span>
                        <span className="w-16 text-right font-mono text-[11.5px] text-slate-700 tabular-nums">
                            <AnimatedNumber value={f.links_sent} />
                        </span>
                        <span className="w-16 text-right font-mono text-[11.5px] text-emerald-600 tabular-nums">
                            <AnimatedNumber value={f.viewers} />
                        </span>
                        <span className="w-16 text-right font-mono text-[11.5px] text-violet-600 tabular-nums hidden md:block">
                            <AnimatedNumber value={f.starters} />
                        </span>
                        <span className="w-16 text-right font-mono text-[11.5px] text-sky-600 tabular-nums">
                            <AnimatedNumber value={f.submissions} />
                        </span>
                        <span className="w-16 text-right font-mono text-[11.5px] text-slate-500 tabular-nums hidden md:block">
                            {rate(f.submissions, f.links_sent)}
                        </span>
                        <span className="w-7 flex justify-end">
                            <Link
                                to="/app/forms/$id"
                                params={{ id: f.form_id }}
                                search={{ tab: "analytics" }}
                                aria-label={`Open ${f.form_name} analytics`}
                                className="size-6 inline-flex items-center justify-center rounded text-slate-400 hover:text-slate-900 hover:bg-slate-100 transition-colors"
                            >
                                <ExternalLinkIcon className="w-3 h-3" />
                            </Link>
                        </span>
                    </div>
                ))}
            </div>
            <p className="px-5 py-2.5 border-t border-slate-200/60 text-[11px] text-slate-400">
                Every recipient gets their own link, so opens and submissions here are tied to the exact contact this
                campaign emailed.
            </p>
        </div>
    );
}
