import React from "react";
import { Loader2Icon } from "lucide-react";
import toast from "react-hot-toast";

import { useContactImports } from "@/lib/api/hooks/app/contacts/useContactImports";
import { isImportActive } from "@/lib/api/models/app/contacts/ContactImport";

// RunningImportChip shows an import still running in the background, for
// anyone in the workspace, and says when it finishes.
export default function RunningImportChip({ onOpen }: { onOpen: (id: string) => void }) {
    const { data } = useContactImports(true, 6);
    const running = (data?.data ?? []).filter((i) => isImportActive(i.status));
    const seen = React.useRef<Set<string>>(new Set());

    // An import this page watched run announces its end once.
    React.useEffect(() => {
        for (const imp of data?.data ?? []) {
            if (isImportActive(imp.status)) {
                seen.current.add(imp.id);
            } else if (seen.current.has(imp.id)) {
                seen.current.delete(imp.id);
                if (imp.status === "completed") {
                    toast.success(
                        `${imp.filename}: ${imp.imported.toLocaleString()} new, ${imp.updated.toLocaleString()} updated` +
                            (imp.failed > 0 ? `, ${imp.failed.toLocaleString()} failed` : ""),
                    );
                }
            }
        }
    }, [data]);

    if (running.length === 0) return null;
    const imp = running[0];
    const pct = imp.total > 0 ? Math.floor((imp.processed / imp.total) * 100) : 0;
    return (
        <button
            type="button"
            onClick={() => onOpen(imp.id)}
            title={`${imp.filename}: ${imp.processed.toLocaleString()} of ${imp.total.toLocaleString()} rows`}
            className="h-7 pl-2 pr-2.5 rounded-md border border-sky-200 bg-sky-50 text-sky-800 text-[12px] font-medium inline-flex items-center gap-1.5 hover:border-sky-300 transition-colors"
        >
            <Loader2Icon className="w-3 h-3 animate-spin" />
            <span className="hidden sm:inline">Importing</span>
            <span className="tabular-nums">{imp.status === "queued" ? "…" : `${pct}%`}</span>
            {running.length > 1 && <span className="text-sky-600">+{running.length - 1}</span>}
        </button>
    );
}
