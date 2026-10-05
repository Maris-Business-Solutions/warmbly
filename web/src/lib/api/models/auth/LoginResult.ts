import type Token from "./Token";

// A login result is one of three things: the token pair, a 2FA challenge, or
// (federated sign-in only) a link challenge: the provider's address already
// belongs to an account with a password, and POST /auth/sso/link needs that
// password before the identity is attached and a session issued.
export interface LoginResult extends Partial<Token> {
    two_fa_required?: boolean;
    pending_token?: string;
    expires_in?: number;
    link_required?: boolean;
    link_email?: string;
    link_provider?: string;
    /** Set when the confirm asked to remember this browser. */
    device_token?: string;
}

// What the login screen needs to run the link step.
export interface SSOLinkChallenge {
    pending_token: string;
    email: string;
    provider: string;
}
