// URL checks for links that come from content another person wrote.

const LINK_SCHEMES = new Set(["http", "https", "mailto", "tel"]);
const SCHEME = /^([a-z][a-z0-9+.-]*):/i;

// Whether an href is relative or uses a scheme a link may open.
export function isSafeLinkHref(href: string): boolean {
    // Browsers ignore control characters and whitespace when reading the scheme.
    // eslint-disable-next-line no-control-regex
    const cleaned = href.replace(/[\u0000- \u007f]/g, "");
    const match = SCHEME.exec(cleaned);
    return !match || LINK_SCHEMES.has(match[1].toLowerCase());
}

// The URL when it is absolute http(s), otherwise null.
export function httpUrl(raw: string | null | undefined): string | null {
    if (!raw) return null;
    try {
        const url = new URL(raw.trim());
        return url.protocol === "http:" || url.protocol === "https:" ? url.href : null;
    } catch {
        return null;
    }
}
