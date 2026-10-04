// HubSpot before it is connected: what HubSpot mode does, and one button.

import { Link } from "@tanstack/react-router";
import { motion } from "framer-motion";
import {
    ArrowLeftIcon,
    BriefcaseIcon,
    ListPlusIcon,
    Loader2Icon,
    LockIcon,
    MailCheckIcon,
    OctagonPauseIcon,
    UserRoundIcon,
} from "lucide-react";

import { HubSpotMark } from "@/components/app/crm/HubSpot";
import useIntegrationCatalog from "@/lib/api/hooks/app/integrations/useIntegrationCatalog";
import { usePermission } from "@/hooks/usePermission";
import { cn } from "@/lib/utils";

import { useHubSpotOAuth } from "./hooks";

const POINTS = [
    {
        icon: MailCheckIcon,
        title: "Activity logged as real emails",
        body: "Sends, replies, bounces and meetings appear on the HubSpot timeline, threaded like any other email.",
    },
    {
        icon: BriefcaseIcon,
        title: "Deals, tasks and notes live in HubSpot",
        body: "Create or edit them in Warmbly and the change lands in HubSpot. Edits in HubSpot show up here.",
    },
    {
        icon: UserRoundIcon,
        title: "Owner and lifecycle in every panel",
        body: "Owner, Lifecycle stage and Lead status sit next to every contact and inbox thread.",
    },
    {
        icon: OctagonPauseIcon,
        title: "Campaigns stop when a deal opens",
        body: "When HubSpot says a contact moved on, Warmbly stops emailing them. No awkward follow-ups.",
    },
    {
        icon: ListPlusIcon,
        title: "Import from HubSpot lists",
        body: "Pull a list straight into a campaign, skipping customers and contacts with open deals.",
    },
];

export default function Hero() {
    const { connect, busy } = useHubSpotOAuth();
    const catalog = useIntegrationCatalog();
    const entry = catalog.data?.catalog.find((e) => e.provider === "hubspot");
    const canManage = usePermission("MANAGE_SETTINGS");
    const notConfigured = !!entry && !entry.configured;
    const blocked = busy || notConfigured || !canManage;

    return (
        <motion.div
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
            className="max-w-3xl mx-auto px-5 py-6 sm:py-10"
        >
            <Link
                to="/app/integrations"
                className="inline-flex items-center gap-1 h-6 -ml-1.5 px-1.5 mb-6 rounded-md text-[11.5px] text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
            >
                <ArrowLeftIcon className="w-3 h-3" />
                Integrations
            </Link>
            <div className="flex items-center gap-3">
                <span className="size-11 rounded-lg bg-orange-50 inline-flex items-center justify-center shrink-0">
                    <HubSpotMark className="w-6 h-6" />
                </span>
                <div className="min-w-0">
                    <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">Integration</div>
                    <h1 className="text-[20px] font-semibold text-slate-900 tracking-tight">Run your CRM on HubSpot</h1>
                </div>
            </div>
            <p className="mt-4 text-[13px] text-slate-600 leading-relaxed max-w-xl">
                Connect HubSpot and Warmbly works on your HubSpot records instead of keeping its own. Your team keeps one
                source of truth, and every email Warmbly sends shows up where your sales team already looks.
            </p>

            <ul className="mt-7 grid sm:grid-cols-2 gap-px bg-slate-200/70 rounded-md border border-slate-200 overflow-hidden">
                {POINTS.map((p, i) => (
                    <motion.li
                        key={p.title}
                        initial={{ opacity: 0, y: 6 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.22, delay: 0.05 + i * 0.04 }}
                        className={cn("bg-white p-4 flex gap-3", i === POINTS.length - 1 && "sm:col-span-2")}
                    >
                        <span className="size-7 rounded-md bg-slate-50 border border-slate-200 inline-flex items-center justify-center shrink-0">
                            <p.icon className="w-3.5 h-3.5 text-slate-600" />
                        </span>
                        <div className="min-w-0">
                            <div className="text-[12.5px] font-medium text-slate-900">{p.title}</div>
                            <p className="text-[11.5px] text-slate-500 leading-relaxed mt-0.5">{p.body}</p>
                        </div>
                    </motion.li>
                ))}
            </ul>

            <div className="mt-7 flex flex-wrap items-center gap-3">
                <button
                    type="button"
                    onClick={() => void connect()}
                    disabled={blocked}
                    className="h-8 px-3.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12.5px] font-medium inline-flex items-center gap-2 transition-colors disabled:opacity-60 disabled:cursor-not-allowed"
                >
                    {busy ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <span className="size-4 rounded-sm bg-white inline-flex items-center justify-center"><HubSpotMark className="w-3 h-3" /></span>}
                    {busy ? "Waiting for HubSpot…" : "Connect HubSpot"}
                </button>
                <span className="text-[11.5px] text-slate-500">Setup takes about two minutes. Every choice has a sensible default.</span>
            </div>

            {!canManage && (
                <p className="mt-3 text-[11.5px] text-amber-700">
                    Only members who can manage workspace settings can connect HubSpot. Ask an admin to set it up.
                </p>
            )}
            {notConfigured && (
                <p className="mt-3 text-[11.5px] text-amber-700">
                    HubSpot is not enabled on this instance yet. An admin needs to add the HubSpot app credentials first.
                </p>
            )}
            <p className="mt-4 text-[10.5px] text-slate-400 flex items-center gap-1">
                <LockIcon className="w-3 h-3" />
                Tokens are encrypted with your workspace key. Nothing changes in HubSpot until you choose to switch.
            </p>
        </motion.div>
    );
}
