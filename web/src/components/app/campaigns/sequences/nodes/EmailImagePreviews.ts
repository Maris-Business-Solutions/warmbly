import { Extension } from "@tiptap/core";
import { DOMSerializer, type Node as ProseMirrorNode } from "@tiptap/pm/model";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet, type MarkViewConstructor } from "@tiptap/pm/view";
import { acquireDashboardImage, directDashboardImage } from "@/lib/dashboardImage";

const TRANSPARENT = "data:image/gif;base64,R0lGODlhAQABAAAAACwAAAAAAQABAAA=";
const CSS_URL = /url\(\s*(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)'|((?:\\.|[^)\\])*))\s*\)/gi;

function cssSource(quoted: string | undefined, single: string | undefined, bare: string | undefined) {
    return (quoted ?? single ?? bare ?? "").trim().replace(/\\([\da-f]{1,6})\s?|\\(.)/gi, (_, hex, character) => {
        if (!hex) return character;
        const point = Number.parseInt(hex, 16);
        return point > 0 && point <= 0x10ffff ? String.fromCodePoint(point) : "\ufffd";
    });
}

function nodeStyle(node: ProseMirrorNode): string {
    const style = String(node.attrs.style ?? "");
    return node.attrs.background && !/background(?:-image)?\s*:/i.test(style)
        ? `${style};background-image:url(${JSON.stringify(node.attrs.background)})`
        : style;
}

// Decorations change only the editing surface, never a template's serialized styles.
export const EmailImagePreviews = Extension.create({
    name: "emailImagePreviews",
    addProseMirrorPlugins() {
        const key = new PluginKey<DecorationSet>("emailImagePreviews");
        const previews = new Map<string, { lease: ReturnType<typeof acquireDashboardImage>; url?: string }>();
        const markViews: Record<string, MarkViewConstructor> = {};
        for (const [name, type] of Object.entries(this.editor.schema.marks)) {
            if (!type.spec.toDOM || !Object.hasOwn(type.spec.attrs ?? {}, "style")) continue;
            markViews[name] = (mark, _, inline) => {
                const style = String(mark.attrs.style ?? "");
                const sources = new Map<string, { lease: ReturnType<typeof acquireDashboardImage>; url?: string }>();
                const previewStyle = () => style.replace(CSS_URL, (match, quoted, single, bare) => {
                    const src = cssSource(quoted, single, bare);
                    return directDashboardImage(src) ? match : `url(${JSON.stringify(sources.get(src)?.url ?? TRANSPARENT)})`;
                });
                const displayMark = mark.type.create({ ...mark.attrs, style: previewStyle() });
                const { dom, contentDOM } = DOMSerializer.renderSpec(document, type.spec.toDOM!(displayMark, inline));
                let alive = true;
                for (const match of style.matchAll(CSS_URL)) {
                    const src = cssSource(match[1], match[2], match[3]);
                    if (directDashboardImage(src) || sources.has(src)) continue;
                    const entry = { lease: acquireDashboardImage(src), url: undefined as string | undefined };
                    sources.set(src, entry);
                    entry.lease.promise.then((url) => {
                        if (alive && dom instanceof HTMLElement) { entry.url = url; dom.setAttribute("style", previewStyle()); }
                    }, () => {});
                }
                return {
                    dom,
                    contentDOM,
                    ignoreMutation: (mutation) => mutation.type === "attributes" && mutation.target === dom && mutation.attributeName === "style",
                    destroy: () => { alive = false; for (const entry of sources.values()) entry.lease.release(); },
                };
            };
        }
        const decorations = (doc: ProseMirrorNode) => {
            const result: Decoration[] = [];
            doc.descendants((node, pos) => {
                if (!node.isBlock) return;
                const original = nodeStyle(node);
                const style = original.replace(CSS_URL, (match, quoted, single, bare) => {
                    const src = cssSource(quoted, single, bare);
                    return directDashboardImage(src) ? match : `url(${JSON.stringify(previews.get(src)?.url ?? TRANSPARENT)})`;
                });
                if (style !== original || node.attrs.background) result.push(Decoration.node(pos, pos + node.nodeSize, { style }));
            });
            return DecorationSet.create(doc, result);
        };
        return [new Plugin({
            key,
            state: {
                init: (_, state) => decorations(state.doc),
                apply: (tr, value) => tr.docChanged || tr.getMeta(key) ? decorations(tr.doc) : value,
            },
            props: { decorations: (state) => key.getState(state), markViews },
            view: (view) => {
                let alive = true;
                const sync = () => {
                    const used = new Set<string>();
                    view.state.doc.descendants((node) => {
                        if (!node.isBlock) return;
                        for (const match of nodeStyle(node).matchAll(CSS_URL)) {
                            const src = cssSource(match[1], match[2], match[3]);
                            if (directDashboardImage(src)) continue;
                            used.add(src);
                            if (previews.has(src)) continue;
                            const entry = { lease: acquireDashboardImage(src), url: undefined as string | undefined };
                            previews.set(src, entry);
                            entry.lease.promise.then((url) => {
                                if (!alive || previews.get(src) !== entry) return;
                                entry.url = url;
                                view.dispatch(view.state.tr.setMeta(key, true).setMeta("addToHistory", false));
                            }, () => {});
                        }
                    });
                    for (const [src, entry] of previews) {
                        if (!used.has(src)) { entry.lease.release(); previews.delete(src); }
                    }
                };
                sync();
                return {
                    update: sync,
                    destroy: () => {
                        alive = false;
                        for (const entry of previews.values()) entry.lease.release();
                        previews.clear();
                    },
                };
            },
        })];
    },
});
