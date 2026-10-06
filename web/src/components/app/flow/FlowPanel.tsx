// FlowPanel is the right-hand editor beside a flow canvas. On desktop it sits
// next to the canvas (which narrows to make room) so the flow stays visible
// while a step is edited; on a phone it covers the canvas.

import React from "react";
import { motion } from "framer-motion";
import { XIcon } from "lucide-react";
import { StepTile, type StepTone } from "./StepCard";
import { cn } from "@/lib/utils";

export default function FlowPanel({
    icon,
    tile,
    tone,
    kicker,
    title,
    onClose,
    tabs,
    footer,
    wide,
    bare,
    children,
}: {
    icon?: React.ReactNode;
    tile?: React.ReactNode;
    tone?: StepTone;
    kicker?: string;
    title?: string;
    onClose?: () => void;
    tabs?: React.ReactNode;
    footer?: React.ReactNode;
    wide?: boolean;
    // Children bring their own header and scrolling.
    bare?: boolean;
    children: React.ReactNode;
}) {
    return (
        <motion.aside
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.12, ease: "easeOut" }}
            className={cn(
                "absolute inset-0 z-20 flex min-h-0 flex-col bg-white md:static md:inset-auto md:z-auto md:shrink-0 md:border-l md:border-slate-200",
                wide ? "md:w-[min(760px,62vw)]" : "md:w-[400px]",
            )}
        >
            {bare ? (
                children
            ) : (
                <>
                    <div className="flex h-14 shrink-0 items-center gap-2.5 border-b border-slate-200 px-3">
                        {tile ??
                            (icon && tone && (
                                <StepTile tone={tone} size="sm">
                                    {icon}
                                </StepTile>
                            ))}
                        <div className="min-w-0 flex-1">
                            {kicker && <div className="truncate text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">{kicker}</div>}
                            <div className="truncate text-[13px] font-semibold text-slate-900">{title}</div>
                        </div>
                        {onClose && (
                            <button
                                type="button"
                                onClick={onClose}
                                aria-label="Close panel"
                                title="Close (Esc)"
                                className="inline-flex size-7 items-center justify-center rounded-md text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700"
                            >
                                <XIcon className="size-4" />
                            </button>
                        )}
                    </div>
                    {tabs}
                    <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
                    {footer && <div className="shrink-0 border-t border-slate-200 px-3 py-2">{footer}</div>}
                </>
            )}
        </motion.aside>
    );
}
