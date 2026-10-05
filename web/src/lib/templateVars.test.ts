import { describe, expect, it } from "vitest";
import { EMAIL_VARIABLES, SENDER_VARS, UNSUBSCRIBE_TOKEN, buildToken, parseToken, tokenLabel, upgradeVariableTokens } from "./templateVars";
import { linkifyUnsubscribe, renderPreview } from "@/components/app/campaigns/sequences/emailPreview";

describe("structured sender fields", () => {
    it("round-trips every mailbox token as a chip", () => {
        for (const v of SENDER_VARS) {
            expect(EMAIL_VARIABLES).toContain(v.token);
            expect(parseToken(v.token)).toEqual({ key: v.key, fallback: null });
            expect(buildToken(v.key)).toBe(v.token);
            expect(tokenLabel(v.token)).toBe(v.label);
            expect(upgradeVariableTokens(`<p>${v.token}</p>`)).toBe(`<p><span data-var="">${v.token}</span></p>`);
        }
    });

    it("retains nested fields when editing their fallback", () => {
        const token = buildToken("Sender.Name", "our team");
        expect(parseToken(token)).toEqual({ key: "Sender.Name", fallback: "our team" });
        expect(upgradeVariableTokens(`<p>${token}</p>`)).toBe(`<p><span data-var="">${token}</span></p>`);
    });

    it("renders sender samples alongside existing contact variables", () => {
        expect(renderPreview("{{.FirstName}}: {{.Sender.Name}} <{{.Sender.Email}}>"))
            .toBe("Alex: Jamie Morgan <jamie@example.com>");
    });

    it("resolves sender fields and conditions against the provided preview context", () => {
        const ctx = { "Sender.Name": "John S.", "Sender.Email": "john@example.com" };
        expect(renderPreview('{{if eq .Sender.Name "John S."}}{{.Sender.Email}}{{else}}wrong{{end}}', ctx))
            .toBe("john@example.com");
        expect(renderPreview('{{if .Sender.Name}}{{.Sender.Name}}{{end}}', ctx)).toBe("John S.");
        expect(renderPreview('{{.Sender.Name | default "our team"}}|{{.Sender.Email}}', {})).toBe("our team|");
    });
});

describe("upgradeVariableTokens", () => {
    it("chips a token in text", () => {
        expect(upgradeVariableTokens("<p>Hi {{.FirstName}}</p>")).toBe('<p>Hi <span data-var="">{{.FirstName}}</span></p>');
    });

    it("leaves a token that is an attribute value alone", () => {
        // A link the author pointed at the unsubscribe variable: wrapping the
        // token in a span here would break the tag.
        const html = `<p><a href="${UNSUBSCRIBE_TOKEN}">no thanks</a></p>`;
        expect(upgradeVariableTokens(html)).toBe(html);
    });

    it("keeps a tag whose attribute contains a >", () => {
        // Reading the quoted ">" as the end of the tag split it, and the href
        // that followed was then wrapped in a chip span.
        const html = `<p><a title="x > y" href="${UNSUBSCRIBE_TOKEN}">no thanks</a></p>`;
        expect(upgradeVariableTokens(html)).toBe(html);
        expect(upgradeVariableTokens(`<p title="a > b">Hi {{.FirstName}}</p>`)).toBe(
            '<p title="a > b">Hi <span data-var="">{{.FirstName}}</span></p>',
        );
    });

    it("is a no-op once the content already carries chips", () => {
        const html = '<p><span data-var="">{{.FirstName}}</span> {{.Company}}</p>';
        expect(upgradeVariableTokens(html)).toBe(html);
    });
});

describe("linkifyUnsubscribe", () => {
    const url = "https://example.com/unsubscribe/preview";

    it("wraps a loose unsubscribe URL in an anchor", () => {
        expect(linkifyUnsubscribe(`<p>Bye. ${url}</p>`)).toBe(`<p>Bye. <a href="${url}">Unsubscribe</a></p>`);
    });

    it("leaves an author's own anchor alone", () => {
        const html = `<p><a href="${url}">no thanks</a></p>`;
        expect(linkifyUnsubscribe(html)).toBe(html);
    });

    it("labels the URL when it is an anchor's own text", () => {
        expect(linkifyUnsubscribe(`<a href="${url}">${url}</a>`)).toBe(`<a href="${url}">Unsubscribe</a>`);
    });

    it("does not rewrite an href behind a quoted > in the same tag", () => {
        const html = `<a title="x > y" href="${url}">read this</a>`;
        expect(linkifyUnsubscribe(html)).toBe(html);
    });

    it("does nothing when the body has no link", () => {
        expect(linkifyUnsubscribe("<p>Hi</p>")).toBe("<p>Hi</p>");
    });
});
