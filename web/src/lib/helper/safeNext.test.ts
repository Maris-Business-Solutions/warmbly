import { describe, expect, it } from "vitest";

import safeNext from "./safeNext";

describe("safeNext", () => {
    it("follows a path on this origin", () => {
        expect(safeNext("/cli?code=ABCD-EFGH", "/home")).toBe("/cli?code=ABCD-EFGH");
        expect(safeNext("/invite/abc", "/home")).toBe("/invite/abc");
    });

    it("refuses anything that can name another host", () => {
        for (const bad of ["//evil.example", "/\\evil.example", "/\\/evil.example", "https://evil.example", "evil", "/a\\b", "/\t/evil.example", "/x\ny", ""]) {
            expect(safeNext(bad, "/home")).toBe("/home");
        }
        expect(safeNext(null, "/home")).toBe("/home");
    });
});
