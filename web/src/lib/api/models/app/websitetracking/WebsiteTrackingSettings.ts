export type WebsiteConsentMode = "explicit" | "implicit";
export type WebsiteLocationPrecision = "none" | "country" | "city";

export interface WebsiteTrackingSettings {
    organization_id: string;
    enabled: boolean;
    site_key: string;
    consent_mode: WebsiteConsentMode;
    location_precision: WebsiteLocationPrecision;
    allowed_hosts: string[];
    retention_days: number;
    updated_at: Date;
    // The deployment's tracking host; empty when the install has none.
    tracking_host: string;
}

export interface UpdateWebsiteTrackingSettings {
    enabled?: boolean;
    consent_mode?: WebsiteConsentMode;
    location_precision?: WebsiteLocationPrecision;
    allowed_hosts?: string[];
    retention_days?: number;
}

export const WEBSITE_RETENTION_MIN_DAYS = 7;
export const WEBSITE_RETENTION_MAX_DAYS = 365;

// The scheme mirrors the backend's TrackingURL: loopback and ported hosts are
// the local tracking service, everything else is https.
export function trackingBaseUrl(host: string): string {
    const h = host.trim();
    if (!h) return "";
    const bare = h.replace(/:\d+$/, "");
    const ported = /:\d+$/.test(h) && !/:443$/.test(h);
    const local = bare === "localhost" || bare.endsWith(".localhost") || /^127\./.test(bare);
    return `${ported || local ? "http" : "https"}://${h}`;
}

export function trackingSnippet(settings: WebsiteTrackingSettings): string {
    const base = trackingBaseUrl(settings.tracking_host) || "https://<your-tracking-host>";
    const consent = settings.consent_mode === "implicit" ? ' data-consent="implicit"' : "";
    return [
        "<script>window.warmbly=window.warmbly||function(){(window.warmbly.q=window.warmbly.q||[]).push(arguments)};</script>",
        `<script async src="${base}/tracking.js" data-site="${settings.site_key}"${consent}></script>`,
    ].join("\n");
}
