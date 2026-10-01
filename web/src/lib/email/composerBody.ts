// A unibox composer body: plain text, or HTML with its plain-text alternative.

import { htmlToPlain } from "@/components/app/campaigns/sequences/emailPreview";
import { plainToHtml } from "./body";

export { htmlToPlain };

export interface ComposerBody {
    /** The text/plain part. In HTML mode with sync on it is derived from `html`. */
    plain: string;
    /** The text/html part; null while the composer is plain text only. */
    html: string | null;
    /** The plain part is generated from the HTML and follows every edit. */
    sync: boolean;
}

/** The plain-text composer's cap, in characters. */
export const MAX_PLAIN_LEN = 4000;
/** The drafts endpoint's cap on each body part, in UTF-8 bytes. */
export const MAX_PART_BYTES = 100_000;

const encoder = new TextEncoder();

/** Whether a part is over what the drafts endpoint stores. */
export function bodyTooLong(b: ComposerBody): boolean {
    return (
        b.html !== null &&
        (encoder.encode(b.html).length > MAX_PART_BYTES || encoder.encode(b.plain).length > MAX_PART_BYTES)
    );
}

/** A plain body keeps the textarea's cap; HTML mode is capped by bodyTooLong. */
export function capPlain(b: ComposerBody): ComposerBody {
    return b.html === null && b.plain.length > MAX_PLAIN_LEN ? { ...b, plain: b.plain.slice(0, MAX_PLAIN_LEN) } : b;
}

/** Whether an HTML body would show the reader anything: text or an image. */
export function htmlHasContent(html: string): boolean {
    return /<img\b/i.test(html) || htmlToPlain(html) !== "";
}

/** In HTML mode the HTML is what the reader sees, so it alone decides. */
export function bodyHasContent(b: ComposerBody): boolean {
    return b.html !== null ? htmlHasContent(b.html) : !!b.plain.trim();
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

/** Appends text (a booking link, a plain template) in whichever mode is on. */
export function withText(b: ComposerBody, text: string): ComposerBody {
    if (!text.trim()) return b;
    if (b.html === null) return capPlain({ ...b, plain: joinPlain(b.plain, text) });
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
    return capPlain({ html: null, plain: b.sync ? htmlToPlain(b.html) : b.plain, sync: false });
}

/** The two parts a send carries. A synced text part is left to the server's renderer. */
export function outgoingParts(b: ComposerBody): { body_plain: string; body_html: string } {
    if (b.html === null) {
        const plain = b.plain.trim();
        return { body_plain: plain, body_html: plainToHtml(plain) };
    }
    return { body_plain: b.sync ? "" : b.plain.trim(), body_html: b.html };
}
