import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({ gmail_oauth_connect: false }));
vi.mock("@/lib/api/hooks/auth/useAuthConfig", () => ({
    default: () => ({ config: auth }),
}));

beforeEach(() => {
    vi.resetModules();
    delete window.__WARMBLY_ENV__;
    vi.stubEnv("VITE_GMAIL_OAUTH_CONNECT", undefined);
    auth.gmail_oauth_connect = false;
});

afterEach(() => {
    vi.unstubAllEnvs();
    delete window.__WARMBLY_ENV__;
});

describe("Google mailbox OAuth deployment capability", () => {
    it.each([
        { name: "defaults off even with backend support", backend: true, expected: false },
        { name: "uses runtime opt-in with backend support", runtime: "true", backend: true, expected: true },
        { name: "respects backend disable", runtime: "true", backend: false, expected: false },
        { name: "uses Vite opt-in in development", build: "true", backend: true, expected: true },
        { name: "runtime off overrides a build-time opt-in", runtime: "false", build: "true", backend: true, expected: false },
        { name: "ignores an invalid frontend setting", runtime: "invalid", backend: true, expected: false },
        { name: "trims a true frontend setting", runtime: " TRUE ", backend: true, expected: true },
        { name: "requires frontend opt-in for Cloud", viaCloud: true, backend: false, expected: false },
        { name: "allows opted-in Cloud-brokered consent", runtime: "true", viaCloud: true, backend: false, expected: true },
    ])("$name", async ({ runtime, build, backend, viaCloud, expected }) => {
        window.__WARMBLY_ENV__ = { GMAIL_OAUTH_CONNECT: runtime };
        vi.stubEnv("VITE_GMAIL_OAUTH_CONNECT", build);
        auth.gmail_oauth_connect = backend;
        const { default: useGmailOAuthConnect } = await import("./useGmailOAuthConnect");
        const { result, unmount } = renderHook(() => useGmailOAuthConnect(viaCloud));
        expect(result.current).toBe(expected);
        unmount();
    });
});
