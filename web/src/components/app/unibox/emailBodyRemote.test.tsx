// Remote content in received mail stays blocked until the reader asks for it,
// so a sender's pixel cannot tell when or where the message was opened.

import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { hasRemoteContent } from "@/lib/email/body";
import EmailBody from "./EmailBody";

const TRACKED = `<p>Hi there</p><img src="https://tracker.example/open.gif" width="1" height="1">`;

function srcDoc(container: HTMLElement): string {
    return container.querySelector("iframe")!.getAttribute("srcdoc")!;
}

function loadButton() {
    return screen.queryByRole("button", { name: /load images/i });
}

describe("EmailBody remote content", () => {
    it("blocks remote images behind a policy and offers to load them", () => {
        const { container } = render(<EmailBody html={TRACKED} blockRemote />);
        expect(srcDoc(container)).toContain(`img-src data:;`);
        expect(srcDoc(container)).not.toContain("https:;");
        expect(loadButton()).toBeInTheDocument();
    });

    it("allows remote images once the reader loads them", () => {
        const { container } = render(<EmailBody html={TRACKED} blockRemote />);
        fireEvent.click(loadButton()!);
        expect(srcDoc(container)).toContain("img-src data: https:");
        expect(loadButton()).not.toBeInTheDocument();
    });

    it("keeps the policy ahead of a whole document's own head", () => {
        const doc = `<!doctype html><html><head><title>x</title></head><body>${TRACKED}</body></html>`;
        const { container } = render(<EmailBody html={doc} blockRemote />);
        const out = srcDoc(container);
        expect(out.indexOf("Content-Security-Policy")).toBeLessThan(out.indexOf("<title>"));
    });

    it("puts the policy in the real head when a comment mentions <head>", () => {
        const doc = `<!-- <head> --><!doctype html><html><head><title>x</title></head><body>${TRACKED}</body></html>`;
        const { container } = render(<EmailBody html={doc} blockRemote />);
        const parsed = new DOMParser().parseFromString(srcDoc(container), "text/html");
        const policy = parsed.head.querySelector('meta[http-equiv="Content-Security-Policy"]');
        expect(policy?.getAttribute("content")).toContain("img-src data:;");
        expect(parsed.head.firstElementChild).toBe(policy);
    });

    it("blocks a new message even after images were loaded for the last one", () => {
        const { container, rerender } = render(<EmailBody html={TRACKED} blockRemote />);
        fireEvent.click(loadButton()!);
        rerender(<EmailBody html={`<p>Next</p><img src="https://other.example/p.gif">`} blockRemote />);
        expect(srcDoc(container)).toContain("img-src data:;");
        expect(loadButton()).toBeInTheDocument();
    });

    it("shows no bar when nothing remote is referenced", () => {
        render(<EmailBody html={`<p>Plain words</p><img src="data:image/png;base64,AAAA">`} blockRemote />);
        expect(loadButton()).not.toBeInTheDocument();
    });

    it("leaves previews of the user's own drafts alone", () => {
        const { container } = render(<EmailBody html={TRACKED} />);
        expect(srcDoc(container)).not.toContain("Content-Security-Policy");
        expect(loadButton()).not.toBeInTheDocument();
    });
});

describe("hasRemoteContent", () => {
    it.each([
        [`<img src="https://a.example/x.png">`, true],
        [`<img src=//a.example/x.png>`, true],
        [`<img srcset="small.png 1x, https://a.example/big.png 2x">`, true],
        [`<td background="http://a.example/bg.png">`, true],
        [`<div style="background-image: url('https://a.example/bg.png')">`, true],
        [`<a href="https://a.example/">link</a>`, false],
        [`<img src="data:image/gif;base64,R0lGOD">`, false],
    ])("%s -> %s", (body, want) => {
        expect(hasRemoteContent(body)).toBe(want);
    });
});
