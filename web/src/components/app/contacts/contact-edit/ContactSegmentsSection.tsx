// Segments panel of the contact drawer: which segments this contact is in,
// with the manual override (pin in / pin out / back to automatic) per row.

import React from "react";
import { CheckIcon, Loader2Icon, MinusIcon, RotateCcwIcon } from "lucide-react";
import { Link } from "@tanstack/react-router";
import toast from "react-hot-toast/headless";

import { useWriteGuard } from "@/hooks/usePermission";
import { useContactSegments, useSetSegmentMembers } from "@/lib/api/hooks/app/segments";
import type { ContactSegment, SegmentMemberMode } from "@/lib/api/models/app/segments/Segment";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { cn } from "@/lib/utils";
import { selectionOf } from "@/lib/api/models/app/contacts/ContactSelection";

export function ContactSegmentsSection({ contactId }: { contactId: string }) {
    const segments = useContactSegments(contactId);
    const set = useSetSegmentMembers();
    const write = useWriteGuard("MANAGE_CONTACTS");
    const [busyId, setBusyId] = React.useState<string | null>(null);

    async function apply(seg: ContactSegment, mode: SegmentMemberMode) {
        if (set.isPending) return;
        setBusyId(seg.id);
        try {
            await set.mutateAsync({ id: seg.id, selection: selectionOf(contactId), mode });
            toast.success(
                mode === "include" ? `Pinned into ${seg.name}` : mode === "exclude" ? `Pinned out of ${seg.name}` : `${seg.name} is automatic again`,
            );
        } catch (err) {
            toast.error(buildError(err as AppError));
        } finally {
            setBusyId(null);
        }
    }

    const list = segments.data ?? [];
    const members = list.filter((s) => s.member);
    const others = list.filter((s) => !s.member);

    return (
        <section>
            <h2 className="text-[10px] uppercase tracking-[0.14em] font-semibold text-slate-500 mb-2 flex items-center gap-2">
                Segments
                {list.length > 0 && <span className="font-mono text-[10px] text-slate-400 tabular-nums normal-case tracking-normal">{members.length} of {list.length}</span>}
            </h2>
            <div className="rounded-md border border-slate-200 bg-white overflow-hidden">
                {segments.isPending ? (
                    <div className="px-3 py-2.5 text-[11.5px] text-slate-400">Loading…</div>
                ) : list.length === 0 ? (
                    <div className="px-3 py-2.5 text-[11.5px] text-slate-400">
                        No segments yet.{" "}
                        <Link to="/app/contacts/segments" className="text-sky-700 hover:underline">
                            Create one
                        </Link>
                        .
                    </div>
                ) : (
                    <ul className="divide-y divide-slate-100">
                        {[...members, ...others].map((s) => {
                            const busy = busyId === s.id;
                            return (
                                <li key={s.id} className="px-3 h-9 flex items-center gap-2">
                                    <span className="size-2 rounded-full shrink-0" style={{ backgroundColor: s.color }} />
                                    <Link
                                        to="/app/contacts/segments/$id"
                                        params={{ id: s.id }}
                                        className={cn("text-[12px] truncate hover:underline", s.member ? "text-slate-900" : "text-slate-400")}
                                    >
                                        {s.name}
                                    </Link>
                                    {s.mode === "include" && (
                                        <span className="inline-flex items-center h-4 px-1 rounded bg-emerald-50 text-emerald-700 text-[10px] font-medium">pinned in</span>
                                    )}
                                    {s.mode === "exclude" && (
                                        <span className="inline-flex items-center h-4 px-1 rounded bg-amber-50 text-amber-700 text-[10px] font-medium">pinned out</span>
                                    )}
                                    <div className="ml-auto flex items-center gap-0.5">
                                        {busy ? (
                                            <Loader2Icon className="w-3 h-3 animate-spin text-slate-400" />
                                        ) : (
                                            <>
                                                {s.mode !== undefined && (
                                                    <IconButton title="Back to automatic" onClick={write.guard(() => apply(s, "auto"))}>
                                                        <RotateCcwIcon className="w-3 h-3" />
                                                    </IconButton>
                                                )}
                                                {s.mode !== "include" && (
                                                    <IconButton title="Pin into segment" onClick={write.guard(() => apply(s, "include"))}>
                                                        <CheckIcon className="w-3 h-3" />
                                                    </IconButton>
                                                )}
                                                {s.mode !== "exclude" && (
                                                    <IconButton title="Pin out of segment" onClick={write.guard(() => apply(s, "exclude"))}>
                                                        <MinusIcon className="w-3 h-3" />
                                                    </IconButton>
                                                )}
                                            </>
                                        )}
                                    </div>
                                </li>
                            );
                        })}
                    </ul>
                )}
            </div>
        </section>
    );
}

function IconButton({ title, onClick, children }: { title: string; onClick: (e: React.MouseEvent<HTMLButtonElement>) => void; children: React.ReactNode }) {
    return (
        <button
            type="button"
            title={title}
            aria-label={title}
            onClick={onClick}
            className="size-6 rounded text-slate-400 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center justify-center transition-colors"
        >
            {children}
        </button>
    );
}
