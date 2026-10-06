// LeadCCBar: the contacts copied on every email one campaign sends this lead,
// so colleagues at one company share a single thread (issue #731). Sits under
// the campaign card's header, next to the hold and pause strips.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Loader2Icon, PlusIcon, SearchIcon, UsersIcon, XIcon } from "lucide-react";
import toast from "react-hot-toast/headless";
import useClickOutside from "@/hooks/useClickOutside";
import useFlipPlacement from "@/hooks/useFlipPlacement";
import useDebouncedValue from "@/hooks/useDebouncedValue";
import { useWriteGuard } from "@/hooks/usePermission";
import useSearchContacts from "@/lib/api/hooks/app/contacts/useSearchContacts";
import { useLeadCCSuggestions, useSetLeadCC } from "@/lib/api/hooks/app/campaigns/useLeadCC";
import { LEAD_CC_MAX, leadCCName, type LeadCC, type LeadCCStatus } from "@/lib/api/models/app/contacts/Contact";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { cn } from "@/lib/utils";

const STATUS_NOTE: Record<Exclude<LeadCCStatus, "active">, string> = {
    unsubscribed: "Unsubscribed, so left off the next email",
    bounced: "Bounced, so left off the next email",
    undeliverable: "Failed verification, so left off the next email",
};

interface Candidate {
    id: string;
    email: string;
    name: string;
    company?: string;
    tag?: string;
}

export default function LeadCCBar({
    campaignId,
    contactId,
    contactName,
    cc,
    sending,
}: {
    campaignId: string;
    contactId: string;
    contactName: string;
    cc: LeadCC[];
    // The lead still has emails to send; copying anyone on a finished flow does nothing.
    sending: boolean;
}) {
    const write = useWriteGuard("MANAGE_CAMPAIGNS");
    const setCC = useSetLeadCC();
    const [open, setOpen] = React.useState(false);

    if (cc.length === 0 && (!write.allowed || !sending)) return null;

    async function save(ids: string[], success: string) {
        try {
            await toast.promise(setCC.mutateAsync({ campaignId, contactId, contactIds: ids }), {
                loading: "Saving…",
                success,
                error: (err: AppError) => buildError(err),
            });
        } catch {
            /* toast.promise already surfaced it */
        }
    }

    const ids = cc.map((c) => c.contact_id);
    const canAdd = write.allowed && sending && cc.length < LEAD_CC_MAX;

    return (
        <div className="px-3 py-2 border-t border-slate-100 flex items-center gap-1.5 flex-wrap">
            <span
                className="text-[10px] uppercase tracking-[0.14em] font-semibold text-slate-400 mr-0.5"
                title={`Copied on every email to ${contactName} in this campaign. A reply from anyone on the thread counts as ${contactName}'s reply.`}
            >
                CC
            </span>
            {cc.map((c) => {
                const note = c.status === "active" ? undefined : STATUS_NOTE[c.status];
                return (
                    <span
                        key={c.contact_id}
                        title={note ? `${c.email} · ${note}` : c.email}
                        className={cn(
                            "inline-flex items-center gap-1 h-5 pl-1.5 rounded bg-slate-100 text-[11px] max-w-[14rem]",
                            write.allowed ? "pr-0.5" : "pr-1.5",
                            note ? "text-slate-400" : "text-slate-700",
                        )}
                    >
                        <span className={cn("truncate", note && "line-through")}>{leadCCName(c)}</span>
                        {write.allowed && (
                            <button
                                type="button"
                                aria-label={`Stop copying ${leadCCName(c)}`}
                                disabled={setCC.isPending}
                                onClick={() => save(ids.filter((id) => id !== c.contact_id), "No longer copied")}
                                className="size-4 inline-flex items-center justify-center rounded text-slate-400 hover:text-slate-700 hover:bg-slate-200 disabled:opacity-50"
                            >
                                <XIcon className="w-2.5 h-2.5" />
                            </button>
                        )}
                    </span>
                );
            })}
            {canAdd && (
                <CCPicker
                    open={open}
                    setOpen={setOpen}
                    campaignId={campaignId}
                    contactId={contactId}
                    contactName={contactName}
                    exclude={ids}
                    busy={setCC.isPending}
                    empty={cc.length === 0}
                    onPick={(id) => {
                        setOpen(false);
                        void save([...ids, id], "Copied on every email to this lead");
                    }}
                />
            )}
        </div>
    );
}

function CCPicker({
    open,
    setOpen,
    campaignId,
    contactId,
    contactName,
    exclude,
    busy,
    empty,
    onPick,
}: {
    open: boolean;
    setOpen: (v: boolean) => void;
    campaignId: string;
    contactId: string;
    contactName: string;
    exclude: string[];
    busy: boolean;
    empty: boolean;
    onPick: (id: string) => void;
}) {
    const ref = React.useRef<HTMLDivElement>(null);
    const triggerRef = React.useRef<HTMLButtonElement>(null);
    const [query, setQuery] = React.useState("");
    useClickOutside(open, () => setOpen(false), ref);
    const placement = useFlipPlacement(triggerRef, open, 300);

    const q = useDebouncedValue(query.trim(), 250);
    const suggestions = useLeadCCSuggestions(campaignId, contactId, open);
    const search = useSearchContacts({
        options: {
            query: q,
            custom_field_filters: [],
            campaign_ids: [],
            sort_by: "updated_at",
            reverse: false,
        },
        limit: 8,
        enabled: open && q.length > 0,
        keepPrevious: true,
    });

    const skip = React.useMemo(() => new Set([contactId, ...exclude]), [contactId, exclude]);
    const candidates: Candidate[] = React.useMemo(() => {
        if (q.length > 0) {
            return (search.contacts ?? [])
                .filter((c) => !skip.has(c.id))
                .map((c) => ({
                    id: c.id,
                    email: c.email,
                    name: `${c.first_name ?? ""} ${c.last_name ?? ""}`.trim() || c.email,
                    company: c.company,
                }));
        }
        return (suggestions.data?.data ?? [])
            .filter((s) => !skip.has(s.contact_id))
            .map((s) => ({
                id: s.contact_id,
                email: s.email,
                name: leadCCName(s),
                company: s.company,
                tag: s.reason === "company" ? "Same company" : "Same domain",
            }));
    }, [q, search.contacts, suggestions.data, skip]);

    const loading = q.length > 0 ? search.isFetching && !search.contacts : suggestions.isLoading;

    return (
        <div
            ref={ref}
            className="relative"
            // On the wrapper, so Escape closes the picker from any focused child.
            onKeyDown={(e) => {
                if (e.key === "Escape" && open) {
                    e.stopPropagation();
                    setOpen(false);
                }
            }}
        >
            <button
                ref={triggerRef}
                type="button"
                disabled={busy}
                onClick={() => setOpen(!open)}
                title={`Copy a colleague on every email to ${contactName} in this campaign`}
                className="inline-flex items-center gap-1 h-5 px-1.5 rounded text-[11px] font-medium border border-dashed border-slate-300 text-slate-500 hover:border-slate-400 hover:text-slate-700 disabled:opacity-50"
            >
                <PlusIcon className="w-2.5 h-2.5" />
                {empty ? "CC a colleague" : "Add"}
            </button>
            <AnimatePresence>
                {open && (
                    <motion.div
                        data-floating
                        initial={{ opacity: 0, y: placement === "top" ? 4 : -4 }}
                        animate={{ opacity: 1, y: 0 }}
                        exit={{ opacity: 0, y: placement === "top" ? 4 : -4 }}
                        transition={{ duration: 0.12 }}
                        className={cn(
                            "absolute left-0 z-30 w-72 rounded-md border border-slate-200 bg-white shadow-[0_12px_32px_-8px_rgba(15,23,42,0.18)] overflow-hidden",
                            placement === "top" ? "bottom-full mb-1" : "top-full mt-1",
                        )}
                    >
                        <div className="px-2 py-1.5 border-b border-slate-200 flex items-center gap-1.5">
                            <SearchIcon className="w-3 h-3 text-slate-400 shrink-0" />
                            <input
                                value={query}
                                onChange={(e) => setQuery(e.target.value)}
                                placeholder="Search contacts…"
                                autoFocus
                                className="w-full h-5 bg-transparent text-[12px] text-slate-900 placeholder:text-slate-400 outline-none"
                            />
                        </div>
                        <div className="max-h-56 overflow-y-auto py-1">
                            {loading ? (
                                <div className="px-3 py-3 flex justify-center">
                                    <Loader2Icon className="w-3.5 h-3.5 animate-spin text-slate-400" />
                                </div>
                            ) : candidates.length === 0 ? (
                                <div className="px-3 py-3 text-[11.5px] text-slate-400 text-center">
                                    {q.length > 0 ? "No matching contacts." : "No colleagues found. Search for a contact."}
                                </div>
                            ) : (
                                candidates.map((c) => (
                                    <button
                                        key={c.id}
                                        type="button"
                                        onClick={() => onPick(c.id)}
                                        className="w-full px-2.5 py-1.5 flex items-center gap-2 text-left hover:bg-slate-100 transition-colors"
                                    >
                                        <UsersIcon className="w-3 h-3 text-slate-400 shrink-0" />
                                        <span className="min-w-0 flex-1">
                                            <span className="block text-[12px] text-slate-800 truncate">{c.name}</span>
                                            <span className="block text-[11px] text-slate-400 truncate">
                                                {c.email}
                                                {c.company ? ` · ${c.company}` : ""}
                                            </span>
                                        </span>
                                        {c.tag && (
                                            <span className="text-[10px] text-sky-700 bg-sky-50 rounded px-1 py-0.5 shrink-0">
                                                {c.tag}
                                            </span>
                                        )}
                                    </button>
                                ))
                            )}
                        </div>
                        <div className="px-2.5 py-1.5 border-t border-slate-100 text-[10.5px] leading-snug text-slate-400">
                            Copied on every email to {contactName} in this campaign, up to {LEAD_CC_MAX}. A reply from
                            anyone counts as {contactName}'s reply.
                        </div>
                    </motion.div>
                )}
            </AnimatePresence>
        </div>
    );
}
