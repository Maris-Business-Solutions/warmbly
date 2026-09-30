// Whether this instance can have Warmbly Cloud serve a root redirect: only a
// self-hosted instance linked to Cloud, while Cloud has room for one more.
import useAuthConfig from "@/lib/api/hooks/auth/useAuthConfig";
import { useCloudLinkStatus } from "@/lib/api/hooks/app/cloudlink/useCloudLink";
import type { PoolLinkRedirectOffer } from "@/lib/api/models/app/cloudlink/CloudLink";
import type { RedirectServer } from "@/lib/api/models/app/emails/SendingDomain";

export interface CloudServing {
    /** Only a self-hosted instance picks; Warmbly Cloud always serves its own. */
    choosable: boolean;
    connected: boolean;
    offer: PoolLinkRedirectOffer | null;
    /** Cloud would take one more redirect for this instance. */
    canServe: boolean;
}

export function useCloudServing(): CloudServing {
    const auth = useAuthConfig();
    const selfHosted = !!auth.data?.self_hosted;
    const status = useCloudLinkStatus(selfHosted, 60_000);
    const offer = status.data?.info?.redirects ?? null;
    const connected = !!status.data?.connected;
    return {
        choosable: selfHosted,
        connected,
        offer,
        canServe: connected && !!offer?.available && offer.used < offer.limit,
    };
}

/** What a new redirect is served from when nobody picked: Cloud when it can, since it needs nothing on this server. */
export function defaultServer(cloud: CloudServing): RedirectServer {
    return cloud.choosable && cloud.canServe ? "cloud" : "instance";
}
