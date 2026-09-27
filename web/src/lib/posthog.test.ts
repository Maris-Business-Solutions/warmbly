import { describe, expect, it } from "vitest";
import { isNoise } from "./posthog";

const exception = (type: string, value: string) => ({
    $exception_list: [{ type, value, mechanism: { handled: false, synthetic: false } }],
});

describe("isNoise", () => {
    it("drops posthog-js's own request timeout", () => {
        expect(isNoise(exception("AbortError", "PostHog request timed out after 3000ms"))).toBe(true);
    });

    it("drops the flattened form of the same timeout", () => {
        expect(isNoise({
            $exception_types: ["AbortError"],
            $exception_values: ["PostHog request timed out after 3000ms"],
        })).toBe(true);
    });

    it("keeps an app AbortError", () => {
        expect(isNoise(exception("AbortError", "signal is aborted without reason"))).toBe(false);
    });

    it("keeps a non-abort error that mentions the timeout", () => {
        expect(isNoise(exception("Error", "PostHog request timed out after 3000ms"))).toBe(false);
    });

    it("drops a passkey ceremony cancelled by leaving the page", () => {
        expect(isNoise(exception("PasskeyCancelled", "Passkey request cancelled"))).toBe(true);
    });
});
