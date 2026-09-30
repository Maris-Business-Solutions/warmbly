// Renders a received message body.
//
// Email HTML is other people's markup: full documents with their own <style>
// blocks, table layouts, and font stacks. Dropping that into the dashboard DOM
// would let a newsletter restyle the app, so it renders inside a sandboxed
// iframe instead. The iframe carries no `allow-scripts`, so nothing in the
// message can execute even though the API already sanitizes the HTML on the
// way out; the two together are belt and braces.
//
// Height is measured from the inner document and kept in sync as images load,
// so the message reads as part of the page rather than a scroll box.
//
// With blockRemote, a Content-Security-Policy in the frame refuses every remote
// image, background and font until the reader asks for them, so a sender's
// tracking pixel cannot learn when, where or in what client a message was read.

import React from "react";
import { ImageOffIcon, MoreHorizontalIcon } from "lucide-react";
import { hasRemoteContent, plainToDisplayHtml } from "@/lib/email/body";

interface EmailBodyProps {
    html?: string | null;
    plain?: string | null;
    // Hold back remote content until the reader loads it. Set for mail other
    // people wrote; previews of the user's own drafts leave it off.
    blockRemote?: boolean;
}

// Inline (data:) images and fonts only: nothing leaves the browser.
const CSP_BLOCKED = "default-src 'none'; img-src data:; font-src data:; style-src 'unsafe-inline'";
// After "Load images": remote images, fonts and media, still no scripts or frames.
const CSP_LOADED =
    "default-src 'none'; img-src data: https: http:; font-src data: https:; media-src https:; style-src 'unsafe-inline'";

// Collapse only recognizable history; ambiguous inline replies stay visible.
const QUOTE_SELECTORS = [
    ".gmail_quote",
    ".gmail_quote_container",
    'blockquote[type="cite"]',
    ".yahoo_quoted",
    ".moz-cite-prefix",
    "#divRplyFwdMsg",
    "#appendonsend",
    "[data-warmbly-quote]",
].join(", ");

const PLAIN_ATTRIBUTION = /^(?:On\b.{0,200}\bwrote:|-{2,}\s*(?:Original Message|Forwarded message)\s*-{2,})$/;

function plainToBody(text: string): string {
    const lines = text.replace(/\r\n/g, "\n").split("\n");
    const at = lines.findIndex((line) => PLAIN_ATTRIBUTION.test(line.trim()) || line.trim().startsWith(">"));
    if (at <= 0 || !lines.slice(0, at).join("").trim()) return plainToDisplayHtml(text);
    // An unprefixed answer after a > quote may be an inline reply, not history.
    const quotedAt = lines.findIndex((line, index) => index >= at && line.trim().startsWith(">"));
    if (quotedAt >= 0 && lines.slice(quotedAt).some((line) => line.trim() && !line.trim().startsWith(">"))) {
        return plainToDisplayHtml(text);
    }
    return `${plainToDisplayHtml(lines.slice(0, at).join("\n"))}<div data-warmbly-quote>${plainToDisplayHtml(lines.slice(at).join("\n"))}</div>`;
}

function prepareQuotes(body: string): { expanded: string; collapsed: string; hasQuote: boolean } {
    const doc = new DOMParser().parseFromString(body, "text/html");
    const quotes = new Set<HTMLElement>(doc.body.querySelectorAll(QUOTE_SELECTORS));
    doc.body.querySelectorAll(".moz-cite-prefix").forEach((marker) => {
        const quote = marker.nextElementSibling;
        if (quote instanceof HTMLElement && quote.tagName === "BLOCKQUOTE") quotes.add(quote);
    });
    // Outlook puts history after its header rather than inside it.
    doc.body.querySelectorAll<HTMLElement>("#divRplyFwdMsg, #appendonsend").forEach((marker) => {
        const tail = doc.createElement("div");
        marker.before(tail);
        while (tail.nextSibling) tail.append(tail.nextSibling);
        quotes.add(tail);
    });
    // Test Outlook tails as a unit, including any text-only siblings.
    quotes.forEach((node) => node.setAttribute("data-warmbly-quote", ""));
    const unquoted = doc.body.cloneNode(true) as HTMLElement;
    unquoted.querySelectorAll("[data-warmbly-quote], script, style").forEach((node) => node.remove());
    const hasContent = !!unquoted.textContent?.trim() || !!unquoted.querySelector("img, hr");
    if (!quotes.size || !hasContent) return { expanded: body, collapsed: body, hasQuote: false };
    const expanded = doc.documentElement.outerHTML;
    quotes.forEach((node) => node.style.setProperty("display", "none", "important"));
    return { expanded, collapsed: doc.documentElement.outerHTML, hasQuote: true };
}

// Typography for the message document. Deliberately minimal: the message
// brings its own styling, and this only sets what it does not. It is injected
// at the END of the document's head so the message's own rules win.
const DOCUMENT_CSS = `
  html, body { margin: 0; padding: 0; }
  body {
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
    font-size: 13px;
    line-height: 1.6;
    color: #1e293b;
    background: transparent;
    word-break: break-word;
    overflow-wrap: anywhere;
  }
  /* Containment, not styling: a 600px design must not scroll the drawer
     sideways, so these hold even against the message's own stylesheet. */
  img { max-width: 100% !important; height: auto; border: 0; }
  table { max-width: 100% !important; }
  a { color: #0284c7; }
  blockquote {
    margin: 0.5em 0;
    padding-left: 0.75em;
    border-left: 2px solid #e2e8f0;
    color: #475569;
  }
  pre { white-space: pre-wrap; }
`;

// A body that is already a whole document (a campaign written in HTML mode, a
// designed newsletter) must not be nested inside another one: the doctype and
// the <head> would land in the body, and the frame would preview something the
// recipient will never see. Its own <head> gets our shell instead.
const DOCUMENT_ROOT = /^\s*(?:<!--[\s\S]*?-->\s*)*(?:<!doctype\s+html|<html[\s>])/i;
const HEAD_OPEN = /<head\b[^>]*>/i;
const HTML_OPEN = /<html\b[^>]*>/i;

function shell(csp: string | null): string {
    // The policy must precede everything else in the head to govern it.
    const policy = csp ? `<meta http-equiv="Content-Security-Policy" content="${csp}">` : "";
    return (
        policy +
        `<meta charset="utf-8"><meta name="referrer" content="no-referrer">` +
        `<base target="_blank"><style>${DOCUMENT_CSS}</style>`
    );
}

function buildDocument(body: string, csp: string | null): string {
    const SHELL = shell(csp);
    if (DOCUMENT_ROOT.test(body)) {
        // Our shell goes FIRST in the head, so the message's own stylesheet
        // comes after it and wins on everything but the containment rules,
        // which are marked !important above.
        if (HEAD_OPEN.test(body)) return body.replace(HEAD_OPEN, `$&${SHELL}`);
        if (HTML_OPEN.test(body)) return body.replace(HTML_OPEN, `$&<head>${SHELL}</head>`);
        return `<!doctype html><html><head>${SHELL}</head>${body}</html>`;
    }
    return `<!doctype html><html><head>${SHELL}</head><body>${body}</body></html>`;
}

export default function EmailBody({ html, plain, blockRemote = false }: EmailBodyProps) {
    const frameRef = React.useRef<HTMLIFrameElement>(null);
    const [height, setHeight] = React.useState(0);
    const [showQuoted, setShowQuoted] = React.useState(false);
    const [remoteLoaded, setRemoteLoaded] = React.useState(false);

    // The message body, before the shell. Split from srcDoc so toggling the
    // quote does not re-run the plain-text conversion.
    const body = React.useMemo(() => {
        const trimmedHtml = (html ?? "").trim();
        if (trimmedHtml) return trimmedHtml;
        const trimmedPlain = (plain ?? "").trim();
        if (trimmedPlain) return plainToBody(trimmedPlain);
        return "";
    }, [html, plain]);

    // A different message starts blocked again.
    React.useEffect(() => setRemoteLoaded(false), [body]);

    const quotes = React.useMemo(() => prepareQuotes(body), [body]);
    const hasQuote = quotes.hasQuote;
    const remote = React.useMemo(() => blockRemote && hasRemoteContent(body), [blockRemote, body]);
    const csp = blockRemote ? (remoteLoaded ? CSP_LOADED : CSP_BLOCKED) : null;
    const srcDoc = React.useMemo(
        () => (body ? buildDocument(showQuoted ? quotes.expanded : quotes.collapsed, csp) : ""),
        [body, quotes, showQuoted, csp],
    );

    // Late-loading remote images change the document height after onLoad, so
    // measurement repeats until the size settles rather than running once.
    const measure = React.useCallback(() => {
        const doc = frameRef.current?.contentDocument;
        if (!doc?.body) return;
        const next = Math.ceil(
            Math.max(doc.body.scrollHeight, doc.documentElement?.scrollHeight ?? 0),
        );
        setHeight((prev) => (Math.abs(prev - next) > 1 ? next : prev));
    }, []);

    const observerRef = React.useRef<ResizeObserver | null>(null);
    React.useEffect(() => () => observerRef.current?.disconnect(), []);

    const onLoad = React.useCallback(() => {
        measure();
        const doc = frameRef.current?.contentDocument;
        if (!doc?.body) return;
        // A reload replaces the document the previous observer watched.
        observerRef.current?.disconnect();
        const observer = new ResizeObserver(measure);
        observer.observe(doc.body);
        observerRef.current = observer;
        doc.querySelectorAll("img").forEach((img) => {
            img.addEventListener("load", measure);
            img.addEventListener("error", measure);
        });
    }, [measure]);

    if (!srcDoc) {
        return (
            <p className="text-[13px] text-slate-400 italic">This message has no content.</p>
        );
    }

    return (
        <>
            {remote && !remoteLoaded && (
                <div className="mb-2 flex items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-2.5 py-1.5 text-[11.5px] text-slate-500">
                    <ImageOffIcon className="w-3.5 h-3.5 shrink-0 text-slate-400" />
                    <span className="flex-1 min-w-0">
                        Remote images are hidden, so the sender can't see when you read this.
                    </span>
                    <button
                        type="button"
                        onClick={() => setRemoteLoaded(true)}
                        className="shrink-0 font-medium text-sky-700 hover:text-sky-800"
                    >
                        Load images
                    </button>
                </div>
            )}
            <iframe
                ref={frameRef}
                title="Message body"
                srcDoc={srcDoc}
                onLoad={onLoad}
                // No allow-scripts: message markup can never run code. allow-popups
                // (plus escape-to-normal-context) is what lets a link actually open.
                sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
                referrerPolicy="no-referrer"
                className="w-full border-0 block"
                style={{ height: height ? `${height}px` : "80px" }}
            />
            {hasQuote && (
                <button
                    type="button"
                    onClick={() => setShowQuoted((v) => !v)}
                    aria-expanded={showQuoted}
                    title={showQuoted ? "Hide the quoted conversation" : "Show the quoted conversation"}
                    className="mt-1 h-5 px-1.5 rounded bg-slate-100 hover:bg-slate-200 text-slate-500 hover:text-slate-700 inline-flex items-center gap-1 text-[10.5px] transition-colors"
                >
                    <MoreHorizontalIcon className="w-3 h-3" />
                    {showQuoted ? "Hide quoted text" : "Show quoted text"}
                </button>
            )}
        </>
    );
}
