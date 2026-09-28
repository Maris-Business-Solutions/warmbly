// A contact on a company domain shows that company's logo. The logo comes from
// DuckDuckGo's favicon service: only the domain is sent, never the address, and
// a missing logo is a 404 the avatar falls back from. It is on for Warmbly Cloud
// and off on a self-host, which makes no outbound call it was not asked to;
// WARMBLY_COMPANY_LOGOS=on or off decides it either way.

import { runtimeEnv } from "@/lib/runtimeConfig";

// Hosts whose inboxes belong to people rather than to a company domain.
const PERSONAL_HOSTS = new Set(["gmail", "outlook", "yahoo", "aol", "icloud", "gmx", "yandex"]);

// Free-mail domains, for contacts whose host is not detected yet.
const PERSONAL_DOMAINS = new Set([
    "gmail.com", "googlemail.com", "outlook.com", "hotmail.com", "hotmail.co.uk", "hotmail.fr", "live.com", "msn.com",
    "yahoo.com", "yahoo.co.uk", "yahoo.fr", "ymail.com", "rocketmail.com", "aol.com", "icloud.com", "me.com", "mac.com",
    "proton.me", "protonmail.com", "pm.me", "gmx.com", "gmx.de", "gmx.net", "web.de", "yandex.com", "yandex.ru",
    "mail.ru", "zohomail.com", "qq.com", "163.com", "126.com", "naver.com", "hey.com", "fastmail.com", "tutanota.com",
]);

// companyLogosEnabled reads the instance's choice; selfHosted is the backend's
// own answer, undefined until it has loaded (logos stay off until then).
export function companyLogosEnabled(selfHosted: boolean | undefined): boolean {
    const setting = runtimeEnv("COMPANY_LOGOS", import.meta.env.VITE_COMPANY_LOGOS, "").trim().toLowerCase();
    if (setting === "on" || setting === "true") return true;
    if (setting === "off" || setting === "false") return false;
    return selfHosted === false;
}

// companyDomainOf is the company domain behind an address, or "" for a
// personal inbox, a reserved test domain, or something that is not an address.
export function companyDomainOf(email: string, mailHost?: string): string {
    const at = email.lastIndexOf("@");
    if (at < 0) return "";
    const domain = email.slice(at + 1).trim().toLowerCase().replace(/\.$/, "");
    if (!/^[a-z0-9.-]+\.[a-z]{2,}$/.test(domain)) return "";
    if (/\.(test|example|invalid|localhost|local)$/.test(domain)) return "";
    if (PERSONAL_DOMAINS.has(domain) || (mailHost && PERSONAL_HOSTS.has(mailHost))) return "";
    return domain;
}

const failed = new Set<string>();

// companyLogoUrl is the logo to try for a domain, or "" when there is none to try.
export function companyLogoUrl(domain: string, enabled: boolean): string {
    if (!domain || !enabled || failed.has(domain)) return "";
    return `https://icons.duckduckgo.com/ip3/${encodeURIComponent(domain)}.ico`;
}

// Remembered for the session, so a domain with no logo is not asked again on every row.
export function markLogoFailed(domain: string) {
    failed.add(domain);
}
