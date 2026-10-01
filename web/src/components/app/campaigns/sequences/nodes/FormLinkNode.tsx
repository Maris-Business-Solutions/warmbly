// FormLinkNode — an atomic inline chip for a personalized form link marker like
// {{form_link:abc123}}. The node keeps the FULL literal token as the span's
// text content, so:
//   - editor.getHTML() -> `<span data-form-link="abc123">{{form_link:abc123}}</span>`,
//     which the Go send-time resolver (internal/models/form_event.go) rewrites
//     into each recipient's personalized URL, and
//   - htmlToPlain()'s naive tag-strip still yields the literal token for
//     body_plain.
// parseHTML matches span[data-form-link]; upgradeVariableTokens
// (@/lib/templateVars) turns previously-saved plain tokens into chips on load.
// Click opens a floating-ui-anchored popover portaled to <body> (same pattern
// as VariableNode) listing the org's published forms to swap the target.

import React from "react";
import { createPortal } from "react-dom";
import { Node as TiptapNode, mergeAttributes, nodeInputRule } from "@tiptap/core";
import { ReactNodeViewRenderer, NodeViewWrapper, type NodeViewProps } from "@tiptap/react";
import { AnimatePresence, motion } from "framer-motion";
import { Link2Icon, XIcon, CheckIcon } from "lucide-react";
import { useForms } from "@/lib/api/hooks/app/forms";
import { useAnchoredFloating } from "@/hooks/useAnchoredFloating";
import { buildFormLinkToken } from "@/lib/templateVars";
import useClickOutside from "@/hooks/useClickOutside";

declare module "@tiptap/core" {
    interface Commands<ReturnType> {
        formLink: {
            // Insert a personalized-form-link chip for the given form public id.
            insertFormLink: (publicId: string) => ReturnType;
        };
    }
}

export const FormLinkNode = TiptapNode.create({
    name: "formLink",
    inline: true,
    group: "inline",
    atom: true,
    selectable: true,

    addAttributes() {
        return {
            publicId: {
                default: "",
                parseHTML: (el) => (el as HTMLElement).getAttribute("data-form-link") || "",
                renderHTML: () => ({}),
            },
        };
    },

    parseHTML() {
        return [
            {
                tag: "span[data-form-link]",
                getAttrs: (el) => {
                    const publicId = ((el as HTMLElement).getAttribute("data-form-link") || "").trim();
                    return publicId ? { publicId } : false;
                },
            },
        ];
    },

    renderHTML({ node, HTMLAttributes }) {
        return [
            "span",
            mergeAttributes(HTMLAttributes, { "data-form-link": node.attrs.publicId }),
            buildFormLinkToken(node.attrs.publicId),
        ];
    },

    renderText({ node }) {
        return buildFormLinkToken(node.attrs.publicId);
    },

    addCommands() {
        return {
            insertFormLink:
                (publicId: string) =>
                ({ chain }) =>
                    chain().insertContent({ type: this.name, attrs: { publicId } }).run(),
        };
    },

    addInputRules() {
        return [
            nodeInputRule({
                find: /\{\{\s*form_link:([a-z0-9]{1,64})\s*\}\}$/,
                type: this.type,
                getAttributes: (match) => ({ publicId: match[1] }),
            }),
        ];
    },

    addNodeView() {
        return ReactNodeViewRenderer(FormLinkChip);
    },
});

// Warning tint for a chip pointing at a missing or unpublished form. Inline
// styles because the base look comes from `.tiptap-body .tpl-var` in global.css
// and a plain class would lose that specificity fight.
const WARN_STYLE: React.CSSProperties = {
    borderColor: "#fde68a", // amber-200
    background: "#fffbeb", // amber-50
    color: "#b45309", // amber-700
};

// Compact chip showing the target form's name, with a click-to-edit popover to
// swap the form or remove the chip. floating-ui keeps it glued through scroll.
function FormLinkChip({ node, updateAttributes, deleteNode, selected }: NodeViewProps) {
    const publicId: string = node.attrs.publicId || "";
    const token = buildFormLinkToken(publicId);
    const { data: forms = [] } = useForms();
    const form = forms.find((f) => f.public_id === publicId);
    const ok = !!form && form.status === "published";
    const [open, setOpen] = React.useState(false);
    const { setReference, setFloating, floatingStyle } = useAnchoredFloating(open, {
        placement: "bottom-start",
        gap: 6,
        maxHeight: true,
    });

    return (
        <NodeViewWrapper as="span" className="tpl-var-wrap">
            <motion.button
                ref={(el) => setReference(el)}
                type="button"
                initial={{ scale: 0.92, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                whileTap={{ scale: 0.96 }}
                transition={{ type: "spring", stiffness: 640, damping: 30 }}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => setOpen((o) => !o)}
                title={ok ? token : "Form not found or unpublished"}
                style={ok ? undefined : WARN_STYLE}
                className={`tpl-var ${selected || open ? "tpl-var-active" : ""}`}
            >
                <Link2Icon className="h-2.5 w-2.5 shrink-0 opacity-70" />
                {form?.name || publicId}
            </motion.button>
            {typeof document !== "undefined" &&
                createPortal(
                    <AnimatePresence>
                        {open && (
                            <FormLinkChipEditor
                                setFloating={setFloating}
                                floatingStyle={floatingStyle}
                                publicId={publicId}
                                onChange={(next) => {
                                    updateAttributes({ publicId: next });
                                    setOpen(false);
                                }}
                                onRemove={() => {
                                    deleteNode();
                                    setOpen(false);
                                }}
                                onClose={() => setOpen(false)}
                            />
                        )}
                    </AnimatePresence>,
                    document.body,
                )}
        </NodeViewWrapper>
    );
}

function FormLinkChipEditor({
    setFloating,
    floatingStyle,
    publicId,
    onChange,
    onRemove,
    onClose,
}: {
    setFloating: (el: HTMLElement | null) => void;
    floatingStyle: React.CSSProperties;
    publicId: string;
    onChange: (next: string) => void;
    onRemove: () => void;
    onClose: () => void;
}) {
    const { data: forms = [] } = useForms();
    const published = forms.filter((f) => f.status === "published");
    const localRef = React.useRef<HTMLDivElement | null>(null);

    const setRefs = React.useCallback(
        (el: HTMLDivElement | null) => {
            localRef.current = el;
            setFloating(el);
        },
        [setFloating],
    );

    useClickOutside(true, onClose, localRef);

    return (
        <motion.div
            ref={setRefs}
            data-floating=""
            style={floatingStyle}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.1 }}
            className="z-[150] w-64 overflow-hidden rounded-lg border border-slate-200 bg-white p-2 text-left shadow-[0_10px_30px_-10px_rgba(15,23,42,0.25)]"
        >
            <div className="px-0.5 pb-1 text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">
                Form
            </div>
            {published.length === 0 ? (
                <div className="px-2 py-1.5 text-[12px] text-slate-500">No published forms yet.</div>
            ) : (
                <div className="max-h-44 space-y-0.5 overflow-y-auto">
                    {published.map((f) => {
                        const active = f.public_id === publicId;
                        return (
                            <button
                                key={f.id}
                                type="button"
                                onMouseDown={(e) => e.preventDefault()}
                                onClick={() => onChange(f.public_id)}
                                className={`flex w-full items-center justify-between gap-2 rounded px-2 py-1 text-left text-[12px] transition-colors ${
                                    active ? "bg-sky-50 text-sky-700" : "text-slate-700 hover:bg-slate-100"
                                }`}
                            >
                                <span className="flex min-w-0 items-center gap-1.5">
                                    <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" />
                                    <span className="truncate">{f.name}</span>
                                </span>
                                {active && <CheckIcon className="h-3 w-3 shrink-0" />}
                            </button>
                        );
                    })}
                </div>
            )}
            <button
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={onRemove}
                className="mt-1.5 flex w-full items-center gap-1.5 rounded px-2 py-1 text-[12px] text-slate-500 transition-colors hover:bg-rose-50 hover:text-rose-600"
            >
                <XIcon className="h-3 w-3" /> Remove
            </button>
        </motion.div>
    );
}
