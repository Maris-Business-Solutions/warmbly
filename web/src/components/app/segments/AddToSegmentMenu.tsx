// "Add to segment" popover for the contacts selection bar: pins the selected
// contacts into a segment as manual includes (or, inside a segment's own
// list, excludes them / clears the override).

import React from "react";
import { Loader2Icon, LayersIcon } from "lucide-react";
import toast from "react-hot-toast/headless";

import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuItem,
    PopoverMenuLabel,
    PopoverMenuTrigger,
} from "@/components/ui/popover-menu";
import { useSegments, useSetSegmentMembers } from "@/lib/api/hooks/app/segments";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import type ContactSelection from "@/lib/api/models/app/contacts/ContactSelection";

export default function AddToSegmentMenu({
    selection,
    count,
    onDone,
}: {
    selection: ContactSelection;
    count: number;
    onDone?: () => void;
}) {
    const segments = useSegments();
    const set = useSetSegmentMembers();

    async function add(id: string, name: string) {
        try {
            const added = await set.mutateAsync({ id, selection, mode: "include" });
            toast.success(`Added ${added.toLocaleString()} contact${added === 1 ? "" : "s"} to ${name}`);
            onDone?.();
        } catch (err) {
            toast.error(buildError(err as AppError));
        }
    }

    const list = segments.data ?? [];
    return (
        <PopoverMenu side="top" align="center">
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    disabled={set.isPending}
                    className="h-7 px-2.5 rounded text-[12px] text-slate-700 hover:text-slate-900 hover:bg-slate-100 font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                >
                    {set.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <LayersIcon className="w-3 h-3" />}
                    <span className="hidden sm:inline">Segment</span>
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={200}>
                <PopoverMenuLabel>Add {count.toLocaleString()} to segment</PopoverMenuLabel>
                {list.length === 0 && (
                    <div className="px-2.5 py-2 text-[11.5px] text-slate-400">No segments yet. Create one under Contacts &gt; Segments.</div>
                )}
                {list.map((s) => (
                    <PopoverMenuItem key={s.id} onSelect={() => add(s.id, s.name)}>
                        <span className="inline-flex items-center gap-2 min-w-0">
                            <span className="size-2 rounded-full shrink-0" style={{ backgroundColor: s.color }} />
                            <span className="truncate">{s.name}</span>
                        </span>
                    </PopoverMenuItem>
                ))}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
