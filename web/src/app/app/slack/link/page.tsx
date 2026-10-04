// /app/slack/link?code=…: where the Warmbly bot in Slack sends a member to
// bind their Slack account to their Warmbly account. Shows both sides, then
// links on an explicit confirm.

import React from "react";
import { Link, useSearchParams } from "react-router-dom";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowRightIcon, BuildingIcon, CheckIcon, ExternalLinkIcon, Loader2Icon, TriangleAlertIcon } from "lucide-react";
import toast from "react-hot-toast";

import { Page, PageBody, PageTopbar } from "@/components/layout/Page";
import ProviderGlyph from "@/app/app/integrations/_components/ProviderGlyph";
import { useConfirmSlackLink, useSlackLinkPreview } from "@/lib/api/hooks/app/integrations/useSlack";
import type { SlackUserLink } from "@/lib/api/models/app/integrations/Slack";
import type { AppError } from "@/lib/api/client/normalizeError";
import { errorMessage } from "@/lib/errors/message";

export default function SlackLinkPage() {
    const [params] = useSearchParams();
    const code = (params.get("code") ?? "").trim();
    const preview = useSlackLinkPreview(code);
    const confirm = useConfirmSlackLink();
    const [linked, setLinked] = React.useState<SlackUserLink | null>(null);

    async function onConfirm() {
        try {
            setLinked(await confirm.mutateAsync(code));
        } catch (err) {
            const e = err as AppError;
            if (e.status === 403) {
                toast.error("You are not a member of that workspace");
            } else if (e.status === 404 || e.code === "slack_link_invalid") {
                toast.error("This link has expired. Ask for a new one in Slack.");
                void preview.refetch();
            } else {
                toast.error(errorMessage(err, "Could not link your Slack account"));
            }
        }
    }

    let body: React.ReactNode;
    if (!code) {
        body = (
            <Problem
                title="This link is missing its code"
                body="Open the link from the message the Warmbly app sent you in Slack, or mention @Warmbly to get a new one."
            />
        );
    } else if (linked) {
        body = <Linked link={linked} orgName={preview.data?.organization_name} />;
    } else if (preview.isPending) {
        body = (
            <div className="py-16 flex justify-center">
                <Loader2Icon className="w-4 h-4 animate-spin text-slate-400" />
            </div>
        );
    } else if (preview.isError || !preview.data) {
        const e = preview.error as unknown as AppError | null;
        const expired = e?.status === 404 || e?.code === "slack_link_invalid";
        body = (
            <Problem
                title={expired ? "This link has expired or was already used" : "Could not open this link"}
                body={
                    expired
                        ? "Links from Slack work once and only for a short time. Mention @Warmbly in Slack to get a new one."
                        : errorMessage(e, "Try again in a moment.")
                }
            />
        );
    } else {
        const p = preview.data;
        body = (
            <div className="space-y-5">
                <div className="space-y-1">
                    <h1 className="text-[15px] font-semibold text-slate-900">Link your Slack account</h1>
                    <p className="text-[12px] text-slate-500 leading-relaxed">
                        The Warmbly assistant will act as you in Slack, with your permissions in this workspace, and
                        can send your notifications as DMs.
                    </p>
                </div>

                <div className="flex flex-col sm:flex-row items-stretch gap-2">
                    <Side
                        label="Slack"
                        title={p.slack_team_name || "Slack workspace"}
                        sub={<span className="font-mono">{p.slack_user_id}</span>}
                        glyph={<ProviderGlyph provider="slack" name="Slack" size={7} />}
                    />
                    <div className="flex items-center justify-center text-slate-300 shrink-0 rotate-90 sm:rotate-0">
                        <ArrowRightIcon className="w-4 h-4" />
                    </div>
                    <Side
                        label="Warmbly"
                        title={p.organization_name}
                        sub="Workspace"
                        glyph={
                            <span className="size-7 rounded-md bg-sky-50 text-sky-600 flex items-center justify-center">
                                <BuildingIcon className="w-3.5 h-3.5" />
                            </span>
                        }
                    />
                </div>

                {!p.is_member && (
                    <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 flex items-start gap-2 text-[11.5px] text-amber-900 leading-relaxed">
                        <TriangleAlertIcon className="w-3.5 h-3.5 mt-0.5 shrink-0 text-amber-500" />
                        <span>
                            You are not a member of {p.organization_name}. Only its members can link a Slack account
                            to it. Ask someone who manages the team to invite you, or sign in with the account that
                            belongs to it.
                        </span>
                    </div>
                )}

                <div className="flex items-center justify-between gap-3">
                    <span className="text-[11px] text-slate-400">
                        Expires {new Date(p.expires_at).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}
                    </span>
                    <button
                        type="button"
                        onClick={() => void onConfirm()}
                        disabled={!p.is_member || confirm.isPending}
                        className="h-8 px-3.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12.5px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {confirm.isPending && <Loader2Icon className="w-3.5 h-3.5 animate-spin" />}
                        Link account
                    </button>
                </div>
            </div>
        );
    }

    return (
        <Page>
            <PageTopbar eyebrow="Slack" subtitle="Link your account" />
            <PageBody>
                <div className="px-5 py-10 flex justify-center">
                    <AnimatePresence mode="wait">
                        <motion.div
                            key={linked ? "done" : "review"}
                            initial={{ opacity: 0, y: 8 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0, y: -8 }}
                            transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
                            className="w-full max-w-[460px] rounded-lg border border-slate-200 bg-white p-5 shadow-[0_8px_24px_-12px_rgba(15,23,42,0.18)]"
                        >
                            {body}
                        </motion.div>
                    </AnimatePresence>
                </div>
            </PageBody>
        </Page>
    );
}

function Side({ label, title, sub, glyph }: { label: string; title: string; sub: React.ReactNode; glyph: React.ReactNode }) {
    return (
        <div className="flex-1 min-w-0 rounded-md border border-slate-200 px-3 py-2.5 flex items-center gap-2.5">
            {glyph}
            <div className="min-w-0">
                <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{label}</div>
                <div className="text-[12.5px] font-medium text-slate-900 truncate">{title}</div>
                <div className="text-[11px] text-slate-500 truncate">{sub}</div>
            </div>
        </div>
    );
}

function Linked({ link, orgName }: { link: SlackUserLink; orgName?: string }) {
    return (
        <div className="space-y-4 text-center">
            <div className="mx-auto size-10 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center">
                <CheckIcon className="w-5 h-5" />
            </div>
            <div className="space-y-1">
                <h1 className="text-[15px] font-semibold text-slate-900">Your Slack account is linked</h1>
                <p className="text-[12px] text-slate-500 leading-relaxed">
                    {orgName ? `Ask Warmbly anything about ${orgName} from Slack.` : "Ask Warmbly anything from Slack."}{" "}
                    The app has sent you a confirmation there.
                </p>
            </div>
            <div className="flex items-center justify-center gap-2">
                <a
                    href={`https://app.slack.com/client/${encodeURIComponent(link.slack_team_id)}`}
                    className="h-8 px-3.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12.5px] font-medium inline-flex items-center gap-1.5 transition-colors"
                >
                    <ExternalLinkIcon className="w-3.5 h-3.5" />
                    Return to Slack
                </a>
                <Link
                    to="/app/integrations"
                    className="h-8 px-3 rounded-md border border-slate-200 hover:border-slate-300 text-[12.5px] text-slate-700 inline-flex items-center transition-colors"
                >
                    Slack settings
                </Link>
            </div>
        </div>
    );
}

function Problem({ title, body }: { title: string; body: string }) {
    return (
        <div className="space-y-3 text-center">
            <div className="mx-auto size-10 rounded-full bg-amber-50 text-amber-600 flex items-center justify-center">
                <TriangleAlertIcon className="w-5 h-5" />
            </div>
            <div className="space-y-1">
                <h1 className="text-[15px] font-semibold text-slate-900">{title}</h1>
                <p className="text-[12px] text-slate-500 leading-relaxed">{body}</p>
            </div>
        </div>
    );
}
