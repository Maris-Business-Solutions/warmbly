// Schema pieces that let the visual editor hold real email markup.
//
// The editor's schema is the contract for what survives a round trip: anything
// it cannot represent is dropped the moment ProseMirror parses the body, and
// the loss is silent. A campaign body pasted from a designed template is table
// layout, inline styles, colours and alignment, so all of that lives here
// (issue #393). Markup a schema can never be faithful to, a whole document
// with its own <head> and <style>, belongs in the HTML source view instead.

import { Extension, Node, mergeAttributes } from "@tiptap/core";
import type { CommandProps, Editor } from "@tiptap/core";
import Paragraph from "@tiptap/extension-paragraph";
import { Table, TableRow, TableCell, TableHeader } from "@tiptap/extension-table";
import { TextStyle } from "@tiptap/extension-text-style";
import { Color } from "@tiptap/extension-text-style/color";
import { BackgroundColor } from "@tiptap/extension-text-style/background-color";
import { FontFamily } from "@tiptap/extension-text-style/font-family";
import { FontSize } from "@tiptap/extension-text-style/font-size";
import { LineHeight } from "@tiptap/extension-text-style/line-height";

// Presentational attributes that carry an email's design. They are declared
// per type rather than globally so a stray attribute cannot land on a node
// that has no business rendering it. `style` is not in the list: it has its
// own definition below, because there may only ever be one producer of it.
const PRESENTATION = ["class", "id", "dir", "title"] as const;

export const BLOCK_ALIGNMENTS = ["left", "center", "right", "justify"] as const;
export type BlockAlign = (typeof BLOCK_ALIGNMENTS)[number];
const ALIGNMENT_SET = new Set<string>(BLOCK_ALIGNMENTS);

// splitDeclarations breaks a style attribute into pairs, respecting the
// parentheses and quotes a value can hold: url(a;b) is one value, not two.
function splitDeclarations(style: string): [string, string][] {
    const out: [string, string][] = [];
    let depth = 0;
    let quote = "";
    let current = "";
    const push = (chunk: string) => {
        const at = chunk.indexOf(":");
        if (at < 0) return;
        const prop = chunk.slice(0, at).trim().toLowerCase();
        const value = chunk.slice(at + 1).trim();
        if (prop && value) out.push([prop, value]);
    };
    for (const ch of style) {
        if (quote) {
            if (ch === quote) quote = "";
        } else if (ch === '"' || ch === "'") {
            quote = ch;
        } else if (ch === "(") {
            depth++;
        } else if (ch === ")") {
            depth = Math.max(0, depth - 1);
        } else if (ch === ";" && depth === 0) {
            push(current);
            current = "";
            continue;
        }
        current += ch;
    }
    push(current);
    return out;
}

function joinDeclarations(decls: [string, string][]): string {
    return decls.map(([prop, value]) => `${prop}: ${value}`).join("; ");
}

// withDeclaration replaces or removes one property, leaving the rest of the
// author's style in the order they wrote it.
function withDeclaration(style: string, prop: string, value: string | null): string {
    const decls = splitDeclarations(style).filter(([p]) => p !== prop);
    if (value) decls.push([prop, value]);
    return joinDeclarations(decls);
}

function readDeclaration(style: string, prop: string): string {
    for (const [p, v] of splitDeclarations(style)) {
        if (p === prop) return v;
    }
    return "";
}

// styleAttribute is the single producer of a node's style attribute.
//
// TipTap merges two style attributes declaration by declaration and splits
// each one on its FIRST colon, so a second producer truncates any value that
// holds a colon of its own: background-image:url(https://cdn/a.png) came back
// as url("https"). Alignment is therefore folded in here rather than taken
// from @tiptap/extension-text-align, which would be that second producer.
function styleAttribute(foldLegacyAlign: boolean) {
    return {
        default: null as string | null,
        parseHTML: (element: HTMLElement) => {
            const own = element.getAttribute("style") ?? "";
            if (!foldLegacyAlign) return own || null;
            // <p align="center"> is how mail has said this since 1997, and the
            // attribute is meaningless on a paragraph in modern HTML.
            const legacy = (element.getAttribute("align") ?? "").trim().toLowerCase();
            if (!legacy || !ALIGNMENT_SET.has(legacy) || readDeclaration(own, "text-align")) {
                return own || null;
            }
            return withDeclaration(own, "text-align", legacy);
        },
        renderHTML: (attributes: Record<string, unknown>) =>
            attributes.style ? { style: attributes.style as string } : {},
    };
}
const TABLE_SIZING = ["width", "height", "align", "valign", "bgcolor", "background"] as const;
const TABLE_LEGACY = ["cellpadding", "cellspacing", "border"] as const;

// passthrough keeps one attribute exactly as it was written. A null default
// means an element that never had it does not gain an empty one.
function passthrough(name: string) {
    return {
        default: null as string | null,
        parseHTML: (element: HTMLElement) => element.getAttribute(name),
        renderHTML: (attributes: Record<string, unknown>) => {
            const value = attributes[name];
            return value ? { [name]: value } : {};
        },
    };
}

function attributesFor(names: readonly string[]) {
    return Object.fromEntries(names.map((name) => [name, passthrough(name)]));
}

// The types that may carry presentation. Table parts are listed separately
// because they also take the legacy sizing attributes email still ships.
// Images are absent on purpose: EmailImageNode composes their style itself
// from width and alignment, and a passthrough would write a second one.
const STYLED_TYPES = ["paragraph", "heading", "bulletList", "orderedList", "listItem", "div", "link"];
const TABLE_TYPES = ["table", "tableRow", "tableCell", "tableHeader"];

// EmailAttributes preserves the presentational attributes of everything the
// schema already knows about. Without it a pasted <td style="padding:16px">
// keeps its cell and loses its padding, which reads as the editor mangling
// the design rather than as a schema limit.
export const EmailAttributes = Extension.create({
    name: "emailAttributes",
    addGlobalAttributes() {
        return [
            {
                types: STYLED_TYPES,
                attributes: { ...attributesFor(PRESENTATION), style: styleAttribute(true) },
            },
            {
                types: TABLE_TYPES,
                attributes: {
                    ...attributesFor(PRESENTATION),
                    ...attributesFor(TABLE_SIZING),
                    // A cell keeps its own align attribute: Outlook reads that
                    // one, so folding it into a style would lose it there.
                    style: styleAttribute(false),
                },
            },
            { types: ["table"], attributes: attributesFor(TABLE_LEGACY) },
        ];
    },

    addCommands() {
        const alignable = new Set([...STYLED_TYPES, ...TABLE_TYPES]);
        const setAlign = (align: BlockAlign | null) => ({ state, dispatch }: CommandProps) => {
            const { from, to } = state.selection;
            const tr = state.tr;
            let changed = false;
            state.doc.nodesBetween(from, to, (node, pos) => {
                if (!alignable.has(node.type.name) || !node.type.spec.attrs?.style) return;
                const current = (node.attrs.style as string | null) ?? "";
                const next = withDeclaration(current, "text-align", align);
                if (next === current) return;
                tr.setNodeMarkup(pos, undefined, { ...node.attrs, style: next || null });
                changed = true;
            });
            if (changed && dispatch) dispatch(tr);
            return changed;
        };
        return {
            setBlockAlign: (align: BlockAlign) => setAlign(align),
            unsetBlockAlign: () => setAlign(null),
        };
    },
});

// blockAlign reads the alignment of the block the caret sits in, for the
// toolbar's pressed state. Empty when the author never set one.
export function blockAlign(editor: Editor): string {
    const { $from } = editor.state.selection;
    for (let depth = $from.depth; depth > 0; depth--) {
        const style = $from.node(depth).attrs?.style as string | undefined;
        if (style) {
            const align = readDeclaration(style, "text-align").toLowerCase();
            if (ALIGNMENT_SET.has(align)) return align;
        }
    }
    return "";
}

// A <div> holding block content. Email templates nest these for layout, and
// dropping them collapsed a two-column design into one run of paragraphs.
export const Div = Node.create({
    name: "div",
    group: "block",
    content: "block+",
    defining: true,
    parseHTML() {
        // Only a div that wraps blocks. One holding a line of text is a
        // paragraph, handled below, so its text does not gain a nested <p>
        // and the margins that come with it.
        return [{ tag: "div", getAttrs: (element) => (hasBlockChild(element) ? {} : false) }];
    },
    renderHTML({ HTMLAttributes }) {
        return ["div", mergeAttributes(HTMLAttributes), 0];
    },
});

// EmailParagraph renders back as whatever tag it was read from. A <div> of
// text stays a <div>: turning it into a <p> adds the client's paragraph
// margins, and a template that spaced its own rows suddenly has gaps.
export const EmailParagraph = Paragraph.extend({
    addAttributes() {
        return {
            ...this.parent?.(),
            htmlTag: {
                default: null as string | null,
                // Rendered through the tag name, never as an attribute.
                renderHTML: () => ({}),
                parseHTML: (element: HTMLElement) =>
                    element.tagName.toLowerCase() === "div" ? "div" : null,
            },
        };
    },
    parseHTML() {
        return [
            { tag: "p" },
            { tag: "div", getAttrs: (element) => (hasBlockChild(element) ? false : {}) },
        ];
    },
    renderHTML({ node, HTMLAttributes }) {
        const tag = node.attrs.htmlTag === "div" ? "div" : "p";
        return [tag, mergeAttributes(this.options.HTMLAttributes, HTMLAttributes), 0];
    },
});

const BLOCK_TAGS = new Set([
    "ADDRESS", "ARTICLE", "ASIDE", "BLOCKQUOTE", "DIV", "DL", "FIELDSET",
    "FIGURE", "FOOTER", "FORM", "H1", "H2", "H3", "H4", "H5", "H6", "HEADER",
    "HR", "OL", "P", "PRE", "SECTION", "TABLE", "UL",
]);

function hasBlockChild(element: HTMLElement): boolean {
    return Array.from(element.children).some((child) => BLOCK_TAGS.has(child.tagName));
}

// Tables are the layout system of HTML email: nothing else holds a column in
// Outlook. resizable is off because a campaign body is not a document: the
// widths belong to the template, and a drag handle would rewrite them.
//
// renderHTML is replaced because TipTap's own emits a <colgroup> and forces a
// "min-width" onto the table. Both are editor furniture: Outlook ignores the
// colgroup, and a min-width nobody asked for appearing on a template's 600px
// table is the editor mangling a design. It is also a third producer of the
// style attribute, which truncates any value holding a colon.
export const EmailTable = Table.extend({
    addNodeView() {
        return null;
    },
    renderHTML({ HTMLAttributes }) {
        return ["table", mergeAttributes(HTMLAttributes), ["tbody", 0]];
    },
}).configure({ resizable: false, allowTableNodeSelection: true });

const emailTableNodes = [EmailTable, TableRow, TableHeader, TableCell];

// The design half of the schema, as one list. Exported whole so the round-trip
// tests mount exactly what the editor does: a test with its own copy of this
// list passes while the editor drops the markup it was meant to protect.
export const emailDesignExtensions = [
    Div,
    ...emailTableNodes,
    TextStyle,
    Color,
    BackgroundColor,
    FontFamily,
    FontSize,
    LineHeight,
    EmailAttributes,
];

declare module "@tiptap/core" {
    interface Commands<ReturnType> {
        emailAttributes: {
            /** Align the blocks the selection covers, folded into their style. */
            setBlockAlign: (align: BlockAlign) => ReturnType;
            /** Drop the alignment, leaving the rest of the style alone. */
            unsetBlockAlign: () => ReturnType;
        };
    }
}
