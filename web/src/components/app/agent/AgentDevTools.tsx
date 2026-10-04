// Dev-only tools in Remie's empty state: a scripted demo run, so the run UI
// can be seen without an AI provider. Lazy-loaded behind import.meta.env.DEV,
// so none of this reaches a production build.

import { PlayIcon } from "lucide-react";

export default function AgentDevTools({ onDemo }: { onDemo: () => void }) {
    return (
        <div className="mt-6 w-full max-w-[340px] rounded-lg border border-dashed border-slate-300 p-3 text-left">
            <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-400">
                Dev preview
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
                <button
                    type="button"
                    onClick={onDemo}
                    className="h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors"
                >
                    <PlayIcon className="size-3" fill="currentColor" />
                    Play demo run
                </button>
                <span className="text-[11px] text-slate-400">
                    or send <code className="font-mono text-slate-500">/demo</code> in any chat
                </span>
            </div>
        </div>
    );
}
