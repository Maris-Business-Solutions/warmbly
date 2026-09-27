import { describe, expect, it } from "vitest";
import { installDomMutationGuard } from "./domGuard";

installDomMutationGuard();

describe("installDomMutationGuard", () => {
    it("skips removing a node another parent now owns", () => {
        const a = document.createElement("div");
        const b = document.createElement("div");
        const text = document.createTextNode("hello");
        b.appendChild(text);

        expect(() => a.removeChild(text)).not.toThrow();
        expect(text.parentNode).toBe(b);
    });

    it("skips inserting before a node another parent now owns", () => {
        const a = document.createElement("div");
        const b = document.createElement("div");
        const ref = document.createTextNode("ref");
        b.appendChild(ref);
        const node = document.createElement("span");

        expect(() => a.insertBefore(node, ref)).not.toThrow();
        expect(node.parentNode).toBeNull();
    });

    it("still removes and inserts normally", () => {
        const a = document.createElement("div");
        const first = document.createElement("i");
        a.appendChild(first);
        const second = document.createElement("b");

        a.insertBefore(second, first);
        expect(a.firstChild).toBe(second);
        a.removeChild(second);
        expect(a.firstChild).toBe(first);
    });
});
