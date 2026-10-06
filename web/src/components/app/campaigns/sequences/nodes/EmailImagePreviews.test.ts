import { afterEach, describe, expect, it, vi } from "vitest";
import { Editor } from "@tiptap/core";
import Document from "@tiptap/extension-document";
import Text from "@tiptap/extension-text";
import Link from "@tiptap/extension-link";
import { EmailImage } from "./EmailImageNode";
import { EmailImagePreviews } from "./EmailImagePreviews";
import { EmailParagraph, emailDesignExtensions } from "./emailHtml";

const { acquire } = vi.hoisted(() => ({ acquire: vi.fn() }));
vi.mock("@/lib/dashboardImage", () => ({ directDashboardImage: (src: string) => src.startsWith("data:") || src.startsWith("/"), acquireDashboardImage: acquire }));
const editors: Editor[] = [];
afterEach(() => { editors.splice(0).forEach((editor) => editor.destroy()); vi.clearAllMocks(); });

function create(html: string) {
    acquire.mockImplementation((src: string) => ({ promise: Promise.resolve(`blob:preview-${encodeURIComponent(src)}`), release: vi.fn() }));
    const editor = new Editor({ injectCSS: false, element: document.createElement("div"), extensions: [Document, Text, EmailParagraph, Link.configure({ openOnClick: false }), EmailImage, ...emailDesignExtensions, EmailImagePreviews], content: html });
    editors.push(editor);
    return editor;
}

describe("display-only email image previews", () => {
    it("renders through a node view without changing saved or sent image URLs, links, alt text or dimensions", async () => {
        const original = "https://customer.example/logo.png?tracking=original";
        const editor = create(`<a href="https://customer.example/click"><img src="${original}" width="300" alt="Logo" data-align="center"></a><p>Hello</p>`);
        await vi.waitFor(() => expect(editor.view.dom.querySelector("img")?.getAttribute("src")).toMatch(/^blob:preview-/));
        editor.commands.setNodeSelection(0);
        editor.commands.updateAttributes("image", { width: 150, align: "right" });
        const saved = editor.getHTML();
        expect(saved).toContain(`src="${original}"`);
        expect(saved).toContain('href="https://customer.example/click"');
        expect(saved).toContain('alt="Logo"');
        expect(saved).toContain('width="150"');
        expect(saved).toContain('data-align="right"');
        expect(saved).not.toContain("blob:");
        expect(editor.getJSON().content?.[0].attrs?.src).toBe(original);
    });

    it("proxies CSS and legacy table background images without changing stored styles", async () => {
        const original = "https://customer.example/background.png";
        const editor = create(`<table background="${original}"><tbody><tr><td style="background-image:url('${original}');color:red"><p>Text</p></td></tr></tbody></table>`);
        await vi.waitFor(() => expect(editor.view.dom.querySelector("td")?.getAttribute("style")).toContain("blob:preview-"));
        expect(editor.view.dom.querySelector("table")?.getAttribute("style")).toContain("blob:preview-");
        editor.commands.setTextSelection(5);
        editor.commands.insertContent("New ");
        expect(editor.getHTML()).toContain(original);
        expect(editor.getHTML()).not.toContain("blob:");
    });

    it("proxies styled links without changing the link or customer HTML", async () => {
        const original = "https://customer.example/button.png";
        const editor = create(`<p><a href="https://customer.example/click" style="background-image:url('${original}')">Visit</a></p>`);
        await vi.waitFor(() => expect(editor.view.dom.querySelector("a")?.getAttribute("style")).toContain("blob:preview-"));
        editor.commands.setTextSelection(3);
        editor.commands.insertContent("today ");
        expect(editor.getHTML()).toContain(original);
        expect(editor.getHTML()).not.toContain("blob:");
    });

    it("does not turn a failed fetch into a saved image rewrite", async () => {
        const editor = create('<img src="https://customer.example/unavailable.png" alt="Logo">');
        editor.destroy();
        editors.pop();
        acquire.mockReturnValue({ promise: Promise.reject(new Error("Unavailable")), release: vi.fn() });
        const failed = new Editor({ element: document.createElement("div"), extensions: [Document, Text, EmailParagraph, EmailImage], content: '<img src="https://customer.example/unavailable.png" alt="Logo">' });
        editors.push(failed);
        await vi.waitFor(() => expect(failed.view.dom.querySelector(".email-image-preview")).toHaveAttribute("data-image-unavailable", "true"));
        expect(failed.getHTML()).toContain('src="https://customer.example/unavailable.png"');
        expect(failed.getHTML()).not.toContain("blob:");
    });
});
