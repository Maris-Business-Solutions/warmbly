// /auth/cli/* — the browser half of `warmbly auth login`.

export type CLIAuthCodeStatus = "pending" | "approved" | "claimed" | "denied";

export interface CLIAuthCode {
    id: string;
    user_code: string;
    client_name: string;
    hostname: string;
    cli_version: string;
    scopes: number;
    scope_names: string[];
    // Present when looked up for a workspace: what approving there grants, capped to your role.
    granted_scopes?: number;
    granted_scope_names?: string[];
    status: CLIAuthCodeStatus;
    organization_id?: string;
    api_key_id?: string;
    expires_at: Date;
    created_at: Date;
}
