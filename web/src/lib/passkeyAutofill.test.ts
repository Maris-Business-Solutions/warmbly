import { afterEach, describe, expect, it, vi } from "vitest";
import { startAuthentication, WebAuthnError } from "@simplewebauthn/browser";
import finish from "@/lib/api/client/auth/passkey/loginFinish";
import { finishPasskeyLogin, PasskeyAutofillUnavailable, type PasskeyLoginChallenge } from "./passkey";

vi.mock("@simplewebauthn/browser", async (importOriginal) => ({
    ...await importOriginal<typeof import("@simplewebauthn/browser")>(),
    startAuthentication: vi.fn(),
}));
vi.mock("@/lib/api/client/auth/passkey/loginFinish", () => ({ default: vi.fn() }));

const challenge = { session: "test-session", options: { publicKey: { challenge: "dGVzdA", rpId: "localhost" } } } as PasskeyLoginChallenge;
afterEach(() => { document.body.innerHTML = ""; vi.clearAllMocks(); });

describe("conditional passkey capabilities", () => {
    it.each([
        new DOMException("Resident credentials or empty allowCredentials lists are not supported at this time.", "NotSupportedError"),
        new Error("Resident credentials or empty allowCredentials lists are not supported at this time."),
        new WebAuthnError({ message: "Unsupported autofill", code: "ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY", cause: new DOMException("Unsupported", "NotSupportedError") }),
    ])("treats unsupported autofill as unavailable without finishing login", async (error) => {
        document.body.innerHTML = '<input autocomplete="username webauthn" />';
        vi.mocked(startAuthentication).mockRejectedValue(error);
        await expect(finishPasskeyLogin(challenge, { conditional: true })).rejects.toBeInstanceOf(PasskeyAutofillUnavailable);
        expect(finish).not.toHaveBeenCalled();
    });

    it("preserves unexpected native errors for reporting", async () => {
        document.body.innerHTML = '<input autocomplete="username webauthn" />';
        const error = new DOMException("Invalid relying party", "SecurityError");
        vi.mocked(startAuthentication).mockRejectedValue(error);
        await expect(finishPasskeyLogin(challenge, { conditional: true })).rejects.toBe(error);
        expect(finish).not.toHaveBeenCalled();
    });

    it("does not classify a wrapped security failure as user cancellation", async () => {
        document.body.innerHTML = '<input autocomplete="username webauthn" />';
        const cause = new DOMException("Invalid relying party", "SecurityError");
        vi.mocked(startAuthentication).mockRejectedValue(new WebAuthnError({ message: "Browser failure", code: "ERROR_PASSTHROUGH_SEE_CAUSE_PROPERTY", cause }));
        await expect(finishPasskeyLogin(challenge, { conditional: true })).rejects.toBe(cause);
    });

    it("does not hide unsupported explicit login", async () => {
        const error = new DOMException("unsupported", "NotSupportedError");
        vi.stubGlobal("PublicKeyCredential", class {});
        Object.defineProperty(navigator, "credentials", { configurable: true, value: { get: vi.fn().mockRejectedValue(error) } });
        await expect(finishPasskeyLogin(challenge)).rejects.toBe(error);
        vi.unstubAllGlobals();
    });
});
