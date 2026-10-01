import { describe, expect, it } from "vitest";
import {
    bodyHasContent,
    bodyTooLong,
    MAX_PART_BYTES,
    MAX_PLAIN_LEN,
    outgoingParts,
    restoreBody,
    toHtmlMode,
    toPlainMode,
    withTemplate,
    withText,
    type ComposerBody,
} from "./composerBody";

const empty: ComposerBody = { plain: "", html: null, sync: false };
const brochureHtml = '<p>Here is our <a href="https://example.com/brochure.pdf">brochure</a>.</p>';
const brochurePlain = "Here is our brochure: https://example.com/brochure.pdf";

describe("withTemplate", () => {
    it("carries a template's HTML body verbatim into an empty composer (#761)", () => {
        const b = withTemplate(empty, { body_plain: brochurePlain, body_html: brochureHtml });
        expect(b).toEqual({ html: brochureHtml, plain: brochurePlain, sync: false });
        expect(outgoingParts(b)).toEqual({ body_html: brochureHtml, body_plain: brochurePlain });
    });

    it("previews a plain part for an HTML-only template and leaves the sent one to the server", () => {
        const b = withTemplate(empty, { body_plain: "", body_html: brochureHtml });
        expect(b.sync).toBe(true);
        expect(b.plain).toBe("Here is our brochure (https://example.com/brochure.pdf).");
        expect(outgoingParts(b)).toEqual({ body_html: brochureHtml, body_plain: "" });
    });

    it("keeps a plain template in a plain composer", () => {
        const b = withTemplate({ ...empty, plain: "Hi Ann," }, { body_plain: "Thanks!", body_html: "" });
        expect(b).toEqual({ plain: "Hi Ann,\n\nThanks!", html: null, sync: false });
    });

    it("treats an empty placeholder HTML body as no HTML", () => {
        expect(withTemplate(empty, { body_plain: "Thanks!", body_html: "<div></div>" }).html).toBeNull();
    });

    it("appends an HTML template under text already typed", () => {
        const b = withTemplate({ ...empty, plain: "Hi Ann," }, { body_plain: brochurePlain, body_html: brochureHtml });
        expect(b.html).toBe(`Hi Ann,<br /><br />${brochureHtml}`);
        expect(b.plain).toBe(`Hi Ann,\n\n${brochurePlain}`);
        expect(b.sync).toBe(false);
    });

    it("appends a plain template to an HTML body as HTML", () => {
        const start = withTemplate(empty, { body_plain: brochurePlain, body_html: brochureHtml });
        const b = withTemplate(start, { body_plain: "See you & thanks", body_html: "" });
        expect(b.html).toBe(`${brochureHtml}<br /><br />See you &amp; thanks`);
        expect(b.plain).toBe(`${brochurePlain}\n\nSee you & thanks`);
    });
});

describe("content and limits", () => {
    it("does not count a stale plain part once the HTML is emptied", () => {
        expect(bodyHasContent({ html: "<br>", plain: brochurePlain, sync: false })).toBe(false);
        expect(bodyHasContent({ html: '<img src="https://x.com/a.png">', plain: "", sync: true })).toBe(true);
    });

    it("measures HTML in bytes, as the drafts endpoint does", () => {
        const wide = "é".repeat(MAX_PART_BYTES / 2 + 1);
        expect(wide.length).toBeLessThan(MAX_PART_BYTES);
        expect(bodyTooLong({ html: `<p>${wide}</p>`, plain: "", sync: true })).toBe(true);
        expect(bodyTooLong({ html: brochureHtml, plain: brochurePlain, sync: false })).toBe(false);
    });

    it("caps the plain body when leaving HTML", () => {
        const long = "a".repeat(MAX_PLAIN_LEN + 50);
        expect(toPlainMode({ html: "<p>x</p>", plain: long, sync: false }).plain).toHaveLength(MAX_PLAIN_LEN);
    });
});

describe("mode switches", () => {
    it("turns typed text into HTML and back", () => {
        const html = toHtmlMode({ ...empty, plain: "a < b\nc" });
        expect(html).toEqual({ html: "a &lt; b<br />c", plain: "a < b\nc", sync: true });
        expect(toPlainMode(html)).toEqual({ html: null, plain: "a < b\nc", sync: false });
    });

    it("keeps an authored plain part when leaving HTML", () => {
        expect(toPlainMode({ html: brochureHtml, plain: brochurePlain, sync: false }).plain).toBe(brochurePlain);
    });

    it("appends text to an HTML body", () => {
        const b = withText({ html: "<p>Hi</p>", plain: "Hi", sync: true }, "Book: https://x.com/m");
        expect(b.html).toBe('<p>Hi</p><br /><br />Book: <a href="https://x.com/m">https://x.com/m</a>');
        expect(b.plain).toBe("Hi\n\nBook: https://x.com/m");
    });
});

describe("restoreBody", () => {
    it("restores a plain draft as plain", () => {
        expect(restoreBody("hello", "")).toEqual({ plain: "hello", html: null, sync: false });
    });

    it("restores sync only when the plain part matches the HTML", () => {
        expect(restoreBody("Hi", "<p>Hi</p>").sync).toBe(true);
        expect(restoreBody(brochurePlain, brochureHtml).sync).toBe(false);
    });
});
