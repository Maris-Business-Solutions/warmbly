import useAuthConfig from "@/lib/api/hooks/auth/useAuthConfig";
import { GMAIL_OAUTH_CONNECT } from "@/lib/information";

export default function useGmailOAuthConnect(viaCloud = false): boolean {
    const { config } = useAuthConfig();
    return GMAIL_OAUTH_CONNECT && (viaCloud || config.gmail_oauth_connect === true);
}
