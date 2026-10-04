// "Suggested for you" in an empty Remie conversation: the problems the Advisor
// measured in this workspace, one line per kind. Choosing one asks Remie to fix
// it, through its own tools and approval cards. Nothing renders when there is
// nothing worth suggesting.

import React from "react";
import { motion } from "framer-motion";
import { ArrowRightIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { SEVERITY_DOT, SEVERITY_RANK } from "@/lib/api/models/app/advisor/Advisor";
import { useAdvisorFindings } from "@/lib/api/hooks/app/advisor/useAdvisor";
import { fixPromptFor, suggestionsFrom } from "./remieTips";

export default function RemieSuggestions({ onPick }: { onPick: (prompt: string) => void }) {
    const { data } = useAdvisorFindings({ limit: 50 });
    const items = React.useMemo(() => suggestionsFrom(data ?? [], SEVERITY_RANK.medium).slice(0, 4), [data]);
    if (items.length === 0) return null;

    return (
        <div className="mt-6 w-full max-w-[340px] text-left">
            <div className="mb-2 px-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-400">
                Suggested for you
            </div>
            <div className="overflow-hidden rounded-xl border border-slate-200 bg-white divide-y divide-slate-100">
                {items.map((s, i) => (
                    <motion.button
                        key={s.key}
                        type="button"
                        initial={{ opacity: 0, y: 4 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ duration: 0.28, delay: 0.04 + i * 0.05, ease: [0.23, 1, 0.32, 1] }}
                        onClick={() => onPick(fixPromptFor(s))}
                        className="group/row flex w-full items-center gap-2.5 px-3 py-2.5 text-left transition-colors hover:bg-slate-50"
                    >
                        <span className={cn("size-1.5 shrink-0 rounded-full", SEVERITY_DOT[s.severity])} />
                        <span className="line-clamp-2 min-w-0 flex-1 text-[12.5px] leading-snug text-slate-700 group-hover/row:text-slate-900">
                            {s.text}
                        </span>
                        <ArrowRightIcon className="size-3.5 shrink-0 text-slate-300 transition-all group-hover/row:translate-x-0.5 group-hover/row:text-sky-600" />
                    </motion.button>
                ))}
            </div>
        </div>
    );
}
