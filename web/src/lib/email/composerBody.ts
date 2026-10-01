// A unibox composer body: plain text, or HTML with its plain-text alternative.

import { plainToHtml } from "./body";

export interface ComposerBody {
    /** The text/plain part. In HTML mode with sync on it is derived from `html`. */
    plain: string;
    /** The text/html part; null while the composer is plain text only. */
    html: string | null;
    /** The plain part is generated from the HTML and follows every edit. */
    sync: boolean;
}

/** Longest HTML body a composer keeps; the drafts endpoint refuses more. */
export const MAX_HTML_LEN = 100_000;

const SKIP = "style, script, title, noscript, template, head, iframe, object";
// Private-use break markers, so a newline in the source collapses like any whitespace.
const BR = "\uE000";
const BREAK1 = "\uE001";
const BREAK2 = "\uE002";
const BLOCK2 = new Set(["P", "BLOCKQUOTE", "TABLE", "UL", "OL", "PRE", "H1", "H2", "H3", "H4", "H5", "H6"]);
const BLOCK1 = new Set(["DIV", "LI", "TR", "SECTION", "ARTICLE", "HEADER", "FOOTER", "ADDRESS", "DT", "DD"]);

/** Renders HTML to its text alternative the way the server's mailhtml.ToPlainText does. */
export function htmlToPlain(html: string): string {
    if (!html.trim()) return "";
    if (typeof DOMParser === "undefined") {
        return html.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
    }
    // DOMParser builds an inert document: nothing is fetched and no handler runs.
    const doc = new DOMParser().parseFromString(html, "text/html");
    doc.querySelectorAll(SKIP).forEach((el) => el.remove());
    doc.querySelectorAll<HTMLElement>("[style]").forEach((el) => {
        if (/display\s*:\s*none/i.test(el.getAttribute("style") ?? "")) el.remove();
    });
    doc.querySelectorAll("a[href]").forEach((a) => {
        const href = (a.getAttribute("href") ?? "").trim();
        if (!/^(https?:|mailto:)/i.test(href)) return;
        const label = (a.textContent ?? "").replace(/\s+/g, " ").trim();
        if (!label) a.append(href.replace(/^mailto:/i, ""));
        else if (label !== href && `mailto:${label}` !== href) a.append(` (${href})`);
    });
    doc.querySelectorAll("br").forEach((br) => br.replaceWith(BR));
    doc.querySelectorAll("li").forEach((li) => li.prepend("- "));
    doc.body.querySelectorAll("*").forEach((el) => {
        const mark = BLOCK2.has(el.tagName) ? BREAK2 : BLOCK1.has(el.tagName) ? BREAK1 : "";
        if (mark) {
            el.prepend(mark);
            el.append(mark);
        } else if (el.tagName === "TD" || el.tagName === "TH") el.append(" ");
    });
    return (doc.body.textContent ?? "")
        .replace(/\s+/g, " ")
        // Adjacent block edges owe the larger break, not the sum.
        .replace(new RegExp(`[ ${BREAK1}${BREAK2}]*[${BREAK1}${BREAK2}][ ${BREAK1}${BREAK2}]*`, "g"), (run) =>
            run.includes(BREAK2) ? "\n\n" : "\n",
        )
        .replace(new RegExp(BR, "g"), "\n")
        .replace(/ *\n */g, "\n")
        .replace(/\n{3,}/g, "\n\n")
        .trim();
}

/** Whether an HTML body would show the reader anything: text or an image. */
export function htmlHasContent(html: string): boolean {
    return /<img\b/i.test(html) || htmlToPlain(html) !== "";
}

export function bodyHasContent(b: ComposerBody): boolean {
    return b.html !== null ? htmlHasContent(b.html) || !!b.plain.trim() : !!b.plain.trim();
}

/** A stored body: HTML mode when it has HTML, synced when the text matches it. */
export function restoreBody(plain: string, html?: string | null): ComposerBody {
    if (!html?.trim()) return { plain, html: null, sync: false };
    return { plain, html, sync: !plain.trim() || plain.trim() === htmlToPlain(html) };
}

function joinPlain(a: string, b: string): string {
    return a.trim() ? `${a.trimEnd()}\n\n${b}` : b;
}

function joinHtml(a: string, b: string): string {
    return htmlHasContent(a) ? `${a.trimEnd()}<br /><br />${b}` : b;
}

/** Edits the HTML; a synced plain part follows it. */
export function withHtml(b: ComposerBody, html: string): ComposerBody {
    return { ...b, html, plain: b.sync ? htmlToPlain(html) : b.plain };
}

/** Appends text (a booking link, a plain template) in whichever mode is on. */
export function withText(b: ComposerBody, text: string): ComposerBody {
    if (!text.trim()) return b;
    if (b.html === null) return { ...b, plain: joinPlain(b.plain, text) };
    const html = joinHtml(b.html, plainToHtml(text));
    return { html, sync: b.sync, plain: b.sync ? htmlToPlain(html) : joinPlain(b.plain, text) };
}

/** Applies a saved template, carrying its HTML body verbatim and its own plain text as the alternative. */
export function withTemplate(b: ComposerBody, t: { body_plain?: string | null; body_html?: string | null }): ComposerBody {
    const tHtml = t.body_html ?? "";
    const tPlain = t.body_plain ?? "";
    if (!htmlHasContent(tHtml)) return withText(b, tPlain);

    if (!bodyHasContent(b)) {
        return tPlain.trim()
            ? { html: tHtml, plain: tPlain, sync: false }
            : { html: tHtml, plain: htmlToPlain(tHtml), sync: true };
    }
    const html = joinHtml(b.html ?? plainToHtml(b.plain), tHtml);
    const sync = !tPlain.trim() && (b.html === null || b.sync);
    return {
        html,
        sync,
        plain: sync ? htmlToPlain(html) : joinPlain(b.plain, tPlain.trim() ? tPlain : htmlToPlain(tHtml)),
    };
}

/** Switches a plain body to HTML, keeping what was typed. */
export function toHtmlMode(b: ComposerBody): ComposerBody {
    if (b.html !== null) return b;
    return { html: plainToHtml(b.plain), plain: b.plain, sync: true };
}

/** Drops the HTML; the plain-text alternative becomes the body. */
export function toPlainMode(b: ComposerBody): ComposerBody {
    if (b.html === null) return b;
    return { html: null, plain: b.sync ? htmlToPlain(b.html) : b.plain, sync: false };
}

/** The two parts a send carries. */
export function outgoingParts(b: ComposerBody): { body_plain: string; body_html: string } {
    if (b.html === null) {
        const plain = b.plain.trim();
        return { body_plain: plain, body_html: plainToHtml(plain) };
    }
    return { body_plain: (b.sync ? htmlToPlain(b.html) : b.plain).trim(), body_html: b.html };
}
