import type Token from "../../models/auth/Token";
import Request from "../Request";

export interface TwoFactorStatus {
    enabled: boolean;
    /** When the authenticator was confirmed. Absent while 2FA is off. */
    confirmed_at?: Date;
    recovery_codes_remaining: number;
    recovery_codes_total: number;
}

export interface TwoFactorEnrollStart {
    secret: string;
    otpauth_uri: string;
    issuer: string;
    account: string;
    algorithm: string;
    digits: number;
    period: number;
}

export interface TwoFactorRecoveryCodes {
    recovery_codes: string[];
}

export async function twoFactorStatus(): Promise<TwoFactorStatus> {
    return await Request<TwoFactorStatus>({
        method: "GET",
        url: "/auth/2fa/status",
        authorization: true,
    });
}

export async function twoFactorEnrollStart(): Promise<TwoFactorEnrollStart> {
    return await Request<TwoFactorEnrollStart>({
        method: "POST",
        url: "/auth/2fa/enroll/start",
        authorization: true,
    });
}

export async function twoFactorEnrollConfirm(code: string): Promise<TwoFactorRecoveryCodes> {
    return await Request<TwoFactorRecoveryCodes>({
        method: "POST",
        url: "/auth/2fa/enroll/confirm",
        data: { code },
        authorization: true,
    });
}

// Replaces every recovery code with a fresh set; the old ones stop working.
export async function twoFactorRegenerateRecoveryCodes(code: string): Promise<TwoFactorRecoveryCodes> {
    return await Request<TwoFactorRecoveryCodes>({
        method: "POST",
        url: "/auth/2fa/recovery-codes",
        data: { code },
        authorization: true,
    });
}

export async function twoFactorDisable(code: string): Promise<void> {
    await Request<void>({
        method: "DELETE",
        url: "/auth/2fa",
        data: { code },
        authorization: true,
    });
}

// Public — the user is not fully authenticated yet (mid-login challenge).
export async function twoFactorVerify(pending_token: string, code: string): Promise<Token> {
    return await Request<Token>({
        method: "POST",
        url: "/auth/2fa/verify",
        data: { pending_token, code },
    });
}
