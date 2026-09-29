// Toolbar controls for the parts of an email that are design rather than
// writing: alignment, colour and table layout.
//
// These exist because the schema holds them now (nodes/emailHtml.ts). A schema
// that can represent a table nobody can insert is only half the feature, and
// the alternative is authoring every design in the HTML source view.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import type { Editor } from "@tiptap/react";
import {
    AlignCenterIcon,
    AlignJustifyIcon,
    AlignLeftIcon,
    AlignRightIcon,
    BaselineIcon,
    CaseSensitiveIcon,
    ChevronDownIcon,
    PaintBucketIcon,
    Table2Icon,
    Trash2Icon,
} from "lucide-react";
import useClickOutside from "@/hooks/useClickOutside";
import { useAnchoredFloating } from "@/hooks/useAnchoredFloating";
import { NumberInput } from "@/components/ui/field";
import { blockAlign, type BlockAlign } from "./nodes/emailHtml";

// The palette a cold email should stay inside. Deliberately short: a colour
// picker with sixteen million options is how a plain-text-looking email that
// lands in the inbox becomes a newsletter that lands in Promotions.
const SWATCHES: { label: string; value: string }[] = [
    { label: "Default", value: "" },
    { label: "Slate", value: "#334155" },
    { label: "Muted", value: "#64748b" },
    { label: "Sky", value: "#0284c7" },
    { label: "Indigo", value: "#4f46e5" },
    { label: "Emerald", value: "#059669" },
    { label: "Amber", value: "#d97706" },
    { label: "Rose", value: "#e11d48" },
    { label: "Black", value: "#0f172a" },
    { label: "White", value: "#ffffff" },
];

// Font stacks every mail client can actually resolve. A web font is not on
// this list on purpose: Gmail and Outlook on Windows never load one, so
// picking it only decides which fallback the reader gets, and it may as well
// be chosen deliberately. Each value is a full stack, because the first name
// is a suggestion and the last one is the guarantee.
const FONT_STACKS: { label: string; value: string }[] = [
    { label: "Default", value: "" },
    { label: "Arial", value: "Arial, Helvetica, sans-serif" },
    { label: "Helvetica", value: "Helvetica, Arial, sans-serif" },
    { label: "Verdana", value: "Verdana, Geneva, sans-serif" },
    { label: "Tahoma", value: "Tahoma, Verdana, sans-serif" },
    { label: "Trebuchet MS", value: "'Trebuchet MS', Tahoma, sans-serif" },
    { label: "Georgia", value: "Georgia, 'Times New Roman', serif" },
    { label: "Times New Roman", value: "'Times New Roman', Times, serif" },
    { label: "Courier New", value: "'Courier New', Courier, monospace" },
];

// Sizes in px: every client honours px, and pt only survives because Outlook
// converts it. 14 to 16 is the band a cold email reads best at.
const FONT_SIZES = [12, 13, 14, 16, 18, 20, 24, 30, 36];
const MIN_FONT_SIZE = 8;
const MAX_FONT_SIZE = 96;

const ALIGNMENTS: { value: BlockAlign; label: string; Icon: typeof AlignLeftIcon }[] = [
    { value: "left", label: "Align left", Icon: AlignLeftIcon },
    { value: "center", label: "Centre", Icon: AlignCenterIcon },
    { value: "right", label: "Align right", Icon: AlignRightIcon },
    { value: "justify", label: "Justify", Icon: AlignJustifyIcon },
];

// Panel is the shared floating surface: portaled, click-away aware and glued
// to its trigger, matching the personalization menu next to it.
function Panel({
    open,
    setOpen,
    title,
    trigger,
    width = 200,
    children,
}: {
    open: boolean;
    setOpen: (open: boolean) => void;
    title: string;
    trigger: React.ReactNode;
    width?: number;
    children: React.ReactNode;
}) {
    const ref = React.useRef<HTMLDivElement>(null);
    useClickOutside(ref, () => setOpen(false));
    const { setReference, setFloating, floatingStyle } = useAnchoredFloating(open, {
        placement: "bottom-start",
        gap: 6,
        maxHeight: true,
    });

    return (
        <div ref={ref} className="relative">
            <button
                ref={(el) => setReference(el)}
                type="button"
                title={title}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => setOpen(!open)}
                className="h-7 px-1.5 inline-flex items-center gap-1 rounded text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900"
            >
                {trigger}
                <ChevronDownIcon className="w-3 h-3" />
            </button>
            {typeof document !== "undefined" &&
                createPortal(
                    <AnimatePresence>
                        {open && (
                            <motion.div
                                ref={setFloating}
                                data-floating=""
                                style={{ ...floatingStyle, width }}
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                exit={{ opacity: 0 }}
                                transition={{ duration: 0.12 }}
                                className="z-[150] max-w-[calc(100vw-24px)] overflow-y-auto rounded-md border border-slate-200 bg-white shadow-[0_12px_32px_-8px_rgba(15,23,42,0.18)]"
                            >
                                <div className="px-3 py-1.5 border-b border-slate-100 text-[10px] uppercase tracking-[0.14em] text-slate-400">
                                    {title}
                                </div>
                                {children}
                            </motion.div>
                        )}
                    </AnimatePresence>,
                    document.body,
                )}
        </div>
    );
}

export function AlignMenu({ editor }: { editor: Editor }) {
    const current = blockAlign(editor);
    const active = ALIGNMENTS.find((a) => a.value === current);
    const Icon = active?.Icon ?? AlignLeftIcon;
    // Pressing the alignment a block already has clears it, so a body can go
    // back to inheriting rather than pinning "left" on every paragraph.
    const apply = (value: BlockAlign) => {
        const chain = editor.chain().focus();
        if (current === value) chain.unsetBlockAlign().run();
        else chain.setBlockAlign(value).run();
    };
    return (
        <div className="flex items-center gap-0.5">
            {ALIGNMENTS.map(({ value, label, Icon: I }) => (
                <button
                    key={value}
                    type="button"
                    title={label}
                    aria-pressed={current === value}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => apply(value)}
                    className={`size-7 hidden items-center justify-center rounded transition-colors sm:inline-flex ${
                        current === value
                            ? "bg-sky-50 text-sky-700"
                            : "text-slate-500 hover:bg-slate-100 hover:text-slate-900"
                    }`}
                >
                    <I className="w-3.5 h-3.5" />
                </button>
            ))}
            {/* Narrow screens get the same four behind one menu rather than a
                toolbar that wraps to three rows. */}
            <div className="sm:hidden">
                <AlignCompact Icon={Icon} onPick={apply} />
            </div>
        </div>
    );
}

function AlignCompact({
    Icon,
    onPick,
}: {
    Icon: typeof AlignLeftIcon;
    onPick: (align: BlockAlign) => void;
}) {
    const [open, setOpen] = React.useState(false);
    return (
        <Panel open={open} setOpen={setOpen} title="Alignment" trigger={<Icon className="w-3.5 h-3.5" />} width={160}>
            <div className="p-1">
                {ALIGNMENTS.map(({ value, label, Icon: I }) => (
                    <button
                        key={value}
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => {
                            onPick(value);
                            setOpen(false);
                        }}
                        className="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-[12px] text-slate-700 transition-colors hover:bg-slate-50"
                    >
                        <I className="w-3.5 h-3.5 text-slate-400" />
                        {label}
                    </button>
                ))}
            </div>
        </Panel>
    );
}

// TypeMenu is the font family and size control. Both write into the same
// <span style>, which is the only place a mail client will read them from.
export function TypeMenu({ editor }: { editor: Editor }) {
    const [open, setOpen] = React.useState(false);
    const attrs = editor.getAttributes("textStyle") as { fontSize?: string; fontFamily?: string };
    const currentSize = parseInt(attrs.fontSize ?? "", 10);
    const size = Number.isFinite(currentSize) ? currentSize : null;
    const family = FONT_STACKS.find((f) => f.value && f.value === attrs.fontFamily);

    // Held separately from the mark so typing "1" on the way to "16" does not
    // apply an 1px size to the selection on every keystroke.
    const [draft, setDraft] = React.useState(size ?? 14);
    React.useEffect(() => setDraft(size ?? 14), [size]);

    const applySize = (px: number | null) => {
        const chain = editor.chain().focus();
        if (px === null) chain.unsetFontSize().run();
        else chain.setFontSize(`${Math.min(Math.max(px, MIN_FONT_SIZE), MAX_FONT_SIZE)}px`).run();
    };

    return (
        <Panel
            open={open}
            setOpen={setOpen}
            title="Font"
            width={236}
            trigger={
                <span className="inline-flex items-center gap-1">
                    <CaseSensitiveIcon className="w-3.5 h-3.5" />
                    {size && <span className="text-[10.5px] tabular-nums text-slate-500">{size}</span>}
                </span>
            }
        >
            <div className="px-2 py-2">
                <div className="px-0.5 pb-1.5 text-[10.5px] text-slate-400">Typeface</div>
                <div className="space-y-0.5">
                    {FONT_STACKS.map((f) => (
                        <button
                            key={f.label}
                            type="button"
                            onMouseDown={(e) => e.preventDefault()}
                            onClick={() => {
                                const chain = editor.chain().focus();
                                if (f.value) chain.setFontFamily(f.value).run();
                                else chain.unsetFontFamily().run();
                                setOpen(false);
                            }}
                            style={f.value ? { fontFamily: f.value } : undefined}
                            className={`flex w-full items-center justify-between rounded px-2 py-1 text-left text-[12.5px] transition-colors hover:bg-slate-50 ${
                                (f.value ? family?.value === f.value : !attrs.fontFamily)
                                    ? "bg-sky-50 text-sky-700"
                                    : "text-slate-700"
                            }`}
                        >
                            {f.label}
                        </button>
                    ))}
                </div>
            </div>
            <div className="border-t border-slate-100 px-2 py-2">
                <div className="px-0.5 pb-1.5 text-[10.5px] text-slate-400">Size</div>
                <div className="flex flex-wrap gap-1">
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => applySize(null)}
                        className={`h-7 rounded border px-2 text-[11.5px] transition-colors ${
                            size === null
                                ? "border-sky-300 bg-sky-50 text-sky-700"
                                : "border-slate-200 text-slate-600 hover:border-sky-300 hover:bg-sky-50/50"
                        }`}
                    >
                        Default
                    </button>
                    {FONT_SIZES.map((px) => (
                        <button
                            key={px}
                            type="button"
                            onMouseDown={(e) => e.preventDefault()}
                            onClick={() => applySize(px)}
                            className={`size-7 rounded border text-[11.5px] tabular-nums transition-colors ${
                                size === px
                                    ? "border-sky-300 bg-sky-50 text-sky-700"
                                    : "border-slate-200 text-slate-600 hover:border-sky-300 hover:bg-sky-50/50"
                            }`}
                        >
                            {px}
                        </button>
                    ))}
                </div>
                <div className="mt-2 flex items-center gap-1.5">
                    <NumberInput
                        value={draft}
                        onChange={setDraft}
                        onCommit={(v) => applySize(v)}
                        min={MIN_FONT_SIZE}
                        max={MAX_FONT_SIZE}
                        suffix="px"
                        className="flex-1"
                    />
                </div>
                <p className="mt-1.5 px-0.5 text-[10.5px] leading-relaxed text-slate-400">
                    Anything under 13px is hard to read on a phone, which is where most cold email is opened.
                </p>
            </div>
        </Panel>
    );
}

export function ColorMenu({ editor }: { editor: Editor }) {
    const [open, setOpen] = React.useState(false);
    const current = (editor.getAttributes("textStyle").color as string | undefined) ?? "";

    const apply = (kind: "color" | "background", value: string) => {
        const chain = editor.chain().focus();
        if (kind === "color") {
            if (value) chain.setColor(value).run();
            else chain.unsetColor().run();
        } else if (value) {
            chain.setBackgroundColor(value).run();
        } else {
            chain.unsetBackgroundColor().run();
        }
        setOpen(false);
    };

    return (
        <Panel
            open={open}
            setOpen={setOpen}
            title="Colour"
            width={216}
            trigger={
                <span className="relative inline-flex">
                    <BaselineIcon className="w-3.5 h-3.5" />
                    <span
                        className="absolute -bottom-0.5 left-0 h-[2px] w-full rounded-sm"
                        style={{ background: current || "#94a3b8" }}
                    />
                </span>
            }
        >
            <Swatches label="Text" icon={<BaselineIcon className="w-3 h-3" />} onPick={(v) => apply("color", v)} />
            <Swatches
                label="Highlight"
                icon={<PaintBucketIcon className="w-3 h-3" />}
                onPick={(v) => apply("background", v)}
            />
        </Panel>
    );
}

function Swatches({
    label,
    icon,
    onPick,
}: {
    label: string;
    icon: React.ReactNode;
    onPick: (value: string) => void;
}) {
    return (
        <div className="px-2 py-2 [&+&]:border-t [&+&]:border-slate-100">
            <div className="flex items-center gap-1.5 px-0.5 pb-1.5 text-[10.5px] text-slate-400">
                {icon}
                {label}
            </div>
            <div className="grid grid-cols-5 gap-1">
                {SWATCHES.map((s) => (
                    <button
                        key={`${label}-${s.value || "none"}`}
                        type="button"
                        title={s.label}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => onPick(s.value)}
                        className="size-7 rounded border border-slate-200 transition-transform hover:scale-105"
                        style={
                            s.value
                                ? { background: s.value }
                                : {
                                      backgroundImage:
                                          "linear-gradient(45deg,transparent 45%,#e11d48 45%,#e11d48 55%,transparent 55%)",
                                  }
                        }
                    />
                ))}
            </div>
        </div>
    );
}

const TABLE_ACTIONS: { label: string; run: (editor: Editor) => void }[] = [
    { label: "Row above", run: (e) => e.chain().focus().addRowBefore().run() },
    { label: "Row below", run: (e) => e.chain().focus().addRowAfter().run() },
    { label: "Column left", run: (e) => e.chain().focus().addColumnBefore().run() },
    { label: "Column right", run: (e) => e.chain().focus().addColumnAfter().run() },
    { label: "Merge or split cells", run: (e) => e.chain().focus().mergeOrSplit().run() },
    { label: "Delete row", run: (e) => e.chain().focus().deleteRow().run() },
    { label: "Delete column", run: (e) => e.chain().focus().deleteColumn().run() },
];

export function TableMenu({ editor }: { editor: Editor }) {
    const [open, setOpen] = React.useState(false);
    const inTable = editor.isActive("table");

    return (
        <Panel
            open={open}
            setOpen={setOpen}
            title={inTable ? "Table" : "Insert table"}
            width={210}
            trigger={<Table2Icon className={`w-3.5 h-3.5 ${inTable ? "text-sky-600" : ""}`} />}
        >
            {!inTable ? (
                <div className="p-2">
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => {
                            editor
                                .chain()
                                .focus()
                                .insertTable({ rows: 2, cols: 2, withHeaderRow: false })
                                .run();
                            setOpen(false);
                        }}
                        className="w-full rounded-md border border-slate-200 px-2 py-1.5 text-[12px] text-slate-700 transition-colors hover:border-sky-300 hover:bg-sky-50/50"
                    >
                        Insert a 2 × 2 table
                    </button>
                    <p className="mt-1.5 px-0.5 text-[10.5px] leading-relaxed text-slate-400">
                        Tables are the only layout Outlook on Windows renders reliably. Header rows are off by
                        default: a layout table has no headings.
                    </p>
                </div>
            ) : (
                <div className="p-1">
                    {TABLE_ACTIONS.map((a) => (
                        <button
                            key={a.label}
                            type="button"
                            onMouseDown={(e) => e.preventDefault()}
                            onClick={() => {
                                a.run(editor);
                                setOpen(false);
                            }}
                            className="w-full rounded px-2 py-1.5 text-left text-[12px] text-slate-700 transition-colors hover:bg-slate-50"
                        >
                            {a.label}
                        </button>
                    ))}
                    <button
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => {
                            editor.chain().focus().deleteTable().run();
                            setOpen(false);
                        }}
                        className="mt-0.5 flex w-full items-center gap-1.5 rounded border-t border-slate-100 px-2 py-1.5 text-left text-[12px] text-rose-600 transition-colors hover:bg-rose-50"
                    >
                        <Trash2Icon className="w-3.5 h-3.5" />
                        Delete table
                    </button>
                </div>
            )}
        </Panel>
    );
}
