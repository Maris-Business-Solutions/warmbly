// FontPicker — the Design rail's font control. A plain dropdown of font names
// tells you nothing about what you are choosing, so every row renders its own
// label in its own typeface, and the trigger previews the current one.
//
// The Google faces are the same ones the hosted page loads, pulled in through
// the shared ensureFont so the preview and the real form never disagree.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { CheckIcon, ChevronDownIcon } from "lucide-react";

import { useAnchoredFloating } from "@/hooks/useAnchoredFloating";

import { FONT_CATALOG, ensureFont, resolveDesign } from "./designCore";
import useClickOutside from "@/hooks/useClickOutside";

export default function FontPicker({
    value,
    onChange,
}: {
    value: string;
    onChange: (v: string) => void;
}) {
    const [open, setOpen] = React.useState(false);
    const panelRef = React.useRef<HTMLDivElement | null>(null);
    const triggerRef = React.useRef<HTMLButtonElement | null>(null);
    const { setReference, setFloating, floatingStyle } = useAnchoredFloating(open, {
        placement: "bottom-start",
        gap: 6,
        maxHeight: true,
        sameWidth: true,
    });

    const current = FONT_CATALOG[value] ? value : "system";

    // Load every face once the list opens so each row previews itself.
    React.useEffect(() => {
        if (!open) return;
        for (const key of Object.keys(FONT_CATALOG)) ensureFont(resolveDesign({ font_family: key }));
    }, [open]);

    useClickOutside(open, () => setOpen(false), [triggerRef, panelRef]);

    const setRefs = React.useCallback(
        (el: HTMLDivElement | null) => {
            panelRef.current = el;
            setFloating(el);
        },
        [setFloating],
    );

    return (
        <>
            <button
                ref={(el) => {
                    triggerRef.current = el;
                    setReference(el);
                }}
                type="button"
                aria-label="Font"
                aria-expanded={open}
                onClick={() => setOpen((o) => !o)}
                className={`h-7 w-full px-2.5 inline-flex items-center gap-2 rounded-md border bg-white text-[12.5px] text-slate-900 transition-colors ${
                    open ? "border-sky-400 ring-2 ring-sky-100" : "border-slate-200 hover:border-slate-300"
                }`}
            >
                <span className="truncate" style={{ fontFamily: FONT_CATALOG[current].stack }}>
                    {FONT_CATALOG[current].label}
                </span>
                <ChevronDownIcon className="w-3 h-3 text-slate-400 ml-auto shrink-0" />
            </button>
            {typeof document !== "undefined" &&
                createPortal(
                    <AnimatePresence>
                        {open && (
                            <motion.div
                                ref={setRefs}
                                data-floating=""
                                style={floatingStyle}
                                // Opacity only: the anchor hook positions with a
                                // transform, which a scale animation would clobber.
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                exit={{ opacity: 0 }}
                                transition={{ duration: 0.12 }}
                                className="z-[70] overflow-y-auto rounded-lg border border-slate-200 bg-white p-1 shadow-[0_12px_32px_-8px_rgba(15,23,42,0.25)]"
                            >
                                {Object.entries(FONT_CATALOG).map(([key, f]) => {
                                    const active = key === current;
                                    return (
                                        <button
                                            key={key}
                                            type="button"
                                            onClick={() => {
                                                onChange(key);
                                                setOpen(false);
                                            }}
                                            className={`w-full h-9 px-2.5 rounded flex items-center gap-2 text-left transition-colors ${
                                                active ? "bg-sky-50 text-sky-700" : "text-slate-700 hover:bg-slate-100"
                                            }`}
                                        >
                                            <span className="truncate text-[14px]" style={{ fontFamily: f.stack }}>
                                                {f.label}
                                            </span>
                                            {active && <CheckIcon className="w-3 h-3 ml-auto shrink-0" />}
                                        </button>
                                    );
                                })}
                            </motion.div>
                        )}
                    </AnimatePresence>,
                    document.body,
                )}
        </>
    );
}
