import { describe, expect, it } from "vitest";
import {
    htmlToPlain,
    outgoingParts,
    restoreBody,
    toHtmlMode,
    toPlainMode,
    withHtml,
    withTemplate,
    withText,
    type ComposerBody,
} from "./composerBody";

const empty: ComposerBody = { plain: "", html: null, sync: false };
const brochureHtml = '<p>Here is our <a href="https://example.com/brochure.pdf">brochure</a>.</p>';
const brochurePlain = "Here is our brochure: https://example.com/brochure.pdf";

describe("htmlToPlain", () => {
    it("keeps paragraphs and puts a link's destination after its label", () => {
        expect(htmlToPlain("<p>One</p><p>Two <a href=\"https://x.com/a\">here</a></p>")).toBe(
            "One\n\nTwo here (https://x.com/a)",
        );
    });

    it("drops stylesheets and hidden preheaders and collapses source whitespace", () => {
        const html = `<html><head><style>p{color:red}</style></head><body>
            <div style="display:none">preheader</div>
            <p>Hello
               there</p></body></html>`;
        expect(htmlToPlain(html)).toBe("Hello there");
    });

    it("breaks lines at <br> and bullets list items", () => {
        expect(htmlToPlain("a<br>b<ul><li>x</li><li>y</li></ul>c")).toBe("a\nb\n\n- x\n- y\n\nc");
        expect(htmlToPlain("<div>one</div><div>two</div>")).toBe("one\ntwo");
    });

    it("does not repeat a link whose label is its address", () => {
        expect(htmlToPlain('<a href="https://x.com">https://x.com</a>')).toBe("https://x.com");
        expect(htmlToPlain('<a href="mailto:a@x.com">a@x.com</a>')).toBe("a@x.com");
    });
});

describe("withTemplate", () => {
    it("carries a template's HTML body verbatim into an empty composer (#761)", () => {
        const b = withTemplate(empty, { body_plain: brochurePlain, body_html: brochureHtml });
        expect(b).toEqual({ html: brochureHtml, plain: brochurePlain, sync: false });
        expect(outgoingParts(b)).toEqual({ body_html: brochureHtml, body_plain: brochurePlain });
    });

    it("derives the plain part from an HTML-only template and keeps it in sync", () => {
        const b = withTemplate(empty, { body_plain: "", body_html: brochureHtml });
        expect(b.sync).toBe(true);
        expect(b.plain).toBe("Here is our brochure (https://example.com/brochure.pdf).");
        const edited = withHtml(b, "<p>Hi</p>");
        expect(edited.plain).toBe("Hi");
    });

    it("keeps a plain template in a plain composer", () => {
        const b = withTemplate({ ...empty, plain: "Hi Ann," }, { body_plain: "Thanks!", body_html: "" });
        expect(b).toEqual({ plain: "Hi Ann,\n\nThanks!", html: null, sync: false });
    });

    it("treats an empty placeholder HTML body as no HTML", () => {
        const b = withTemplate(empty, { body_plain: "Thanks!", body_html: "<div></div>" });
        expect(b.html).toBeNull();
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

describe("mode switches", () => {
    it("turns typed text into HTML and back", () => {
        const html = toHtmlMode({ ...empty, plain: "a < b\nc" });
        expect(html).toEqual({ html: "a &lt; b<br />c", plain: "a < b\nc", sync: true });
        expect(toPlainMode(html)).toEqual({ html: null, plain: "a < b\nc", sync: false });
    });

    it("keeps an authored plain part when leaving HTML", () => {
        const b: ComposerBody = { html: brochureHtml, plain: brochurePlain, sync: false };
        expect(toPlainMode(b).plain).toBe(brochurePlain);
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
