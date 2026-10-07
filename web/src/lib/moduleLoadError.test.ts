import { describe, expect, it } from "vitest";
import { isModuleLoadError } from "./moduleLoadError";

describe("module-load recovery", () => {
    it.each([
        "Failed to fetch dynamically imported module: https://app.warmbly.com/assets/layout.js",
        "error loading dynamically imported module: https://app.warmbly.com/assets/layout.js",
        "Importing a module script failed.",
        `'text/html' is not a valid JavaScript MIME type for module script '${window.location.origin}/assets/layout-aTnJVElG.js'.`,
    ])("recognizes module load failures: %s", (message) => {
        expect(isModuleLoadError(new TypeError(message))).toBe(true);
    });

    it.each([
        new SyntaxError("Unterminated regular expression literal '/'"),
        new SyntaxError("Unexpected end of script"),
        new Error("Failed to fetch dynamically imported module"),
        new TypeError("Failed to fetch"),
        new TypeError("Cannot read properties of undefined"),
        new TypeError("'text/html' is not a valid JavaScript MIME type for module script 'https://other.example/assets/layout.js'."),
        new TypeError(`'text/html' is not a valid JavaScript MIME type for module script '${window.location.origin}/api/script.js'.`),
    ])("does not reload unrelated errors: %s", (error) => {
        expect(isModuleLoadError(error)).toBe(false);
    });
});
