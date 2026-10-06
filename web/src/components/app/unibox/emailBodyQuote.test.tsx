// The quote split is the one bit of real logic in EmailBody: get it wrong and
// either the whole reply disappears behind the toggle, or the toggle never
// appears and the thread stays a wall of repeated text.

import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import EmailBody from "./EmailBody";

const GMAIL_REPLY =
    `<div dir="ltr">Sounds good, Tuesday works.</div>` +
    `<div class="gmail_quote"><blockquote class="gmail_quote">Original question here</blockquote></div>`;

const PLAIN_REPLY = [
    "Sounds good, Tuesday works.",
    "",
    "On Mon, 15 Sep 2026 at 09:12, Ada <ada@example.com> wrote:",
    "> Can we move the call?",
].join("\n");

function toggle() {
    return screen.queryByRole("button", { name: /quoted text/i });
}

function frameDocument(container: HTMLElement) {
    const frame = container.querySelector("iframe")!;
    const doc = frame.contentDocument!;
    if (!doc.documentElement) doc.appendChild(doc.createElement("html"));
    fireEvent.load(frame);
    return doc;
}

describe("EmailBody quoted history", () => {
    it("offers the toggle for an HTML reply that carries a quote", () => {
        render(<EmailBody html={GMAIL_REPLY} />);
        expect(toggle()).toHaveTextContent("Show quoted text");
    });

    it("flips the toggle label on click", () => {
        render(<EmailBody html={GMAIL_REPLY} />);
        fireEvent.click(toggle()!);
        expect(toggle()).toHaveTextContent("Hide quoted text");
    });

    it("detects a plain-text quote tail and wraps it", () => {
        const { container } = render(<EmailBody plain={PLAIN_REPLY} />);
        expect(toggle()).toBeInTheDocument();
        const srcDoc = frameDocument(container).documentElement.outerHTML;
        expect(srcDoc).toContain("Sounds good, Tuesday works.");
        // The tail is inside the wrapper the hide rule targets, the new line is not.
        // lastIndexOf: the attribute also appears in the hide rule up in <head>.
        const wrapped = srcDoc.slice(srcDoc.lastIndexOf("data-warmbly-quote"));
        expect(wrapped).toContain("Can we move the call?");
        expect(wrapped).not.toContain("Sounds good");
    });

    it("leaves a message that is nothing but quote alone", () => {
        render(<EmailBody plain={"> only the quote\n> second line"} />);
        expect(toggle()).not.toBeInTheDocument();
    });

    it("shows no toggle for an ordinary first message", () => {
        render(<EmailBody plain={"Hi Ada,\n\nAre you free Tuesday?"} />);
        expect(toggle()).not.toBeInTheDocument();
    });
    it.each([
        '<div class="gmail_quote_container">Old reply</div>',
        '<blockquote type = cite>Old reply</blockquote>',
        '<div class="yahoo_quoted">Old reply</div>',
        '<div class="moz-cite-prefix">On Monday, Ada wrote:</div><blockquote type="cite">Old reply</blockquote>',
        '<div class="moz-cite-prefix">On Monday:</div><blockquote>Old reply</blockquote>',
        '<div id="divRplyFwdMsg">From: Ada</div><p>Old reply</p>',
        '<div id="appendonsend"></div>Old reply',
    ])("hides and restores the same HTML history: %s", (quote) => {
        const { container } = render(<EmailBody html={`<p>New reply</p>${quote}`} />);
        const documentBody = () => frameDocument(container).body.cloneNode(true) as HTMLBodyElement;
        const visibleText = () => {
            const doc = documentBody();
            doc.querySelectorAll<HTMLElement>('[style]').forEach((node) => {
                if (node.style.display === "none") node.remove();
            });
            return doc.textContent;
        };
        expect(toggle()).toHaveAttribute("aria-expanded", "false");
        expect(visibleText()).toContain("New reply");
        expect(visibleText()).not.toContain("Old reply");
        fireEvent.click(toggle()!);
        expect(toggle()).toHaveAttribute("aria-expanded", "true");
        expect(visibleText()).toContain("Old reply");
        expect(container.querySelector("iframe")!.getAttribute("sandbox")).not.toContain("allow-scripts");
    });

    it("keeps quote-only HTML and inline answers visible", () => {
        const { rerender } = render(<EmailBody html='<div class="gmail_quote">The entire message</div>' />);
        expect(toggle()).toBeNull();
        rerender(<EmailBody plain={"Intro\n> question one\nMy answer\n> question two\nAnother answer"} />);
        expect(toggle()).toBeNull();
        rerender(<EmailBody plain={"Intro\nOn Monday Ada wrote:\n> question\nMy inline answer"} />);
        expect(toggle()).toBeNull();
    });

    it("does not mistake ordinary From lines or Gmail extra wrappers for history", () => {
        const { rerender } = render(<EmailBody plain={"Shipping details\nFrom: London\nTo: Paris"} />);
        expect(toggle()).toBeNull();
        rerender(<EmailBody html='<div class="gmail_extra">My new reply</div>' />);
        expect(toggle()).toBeNull();
    });

});
