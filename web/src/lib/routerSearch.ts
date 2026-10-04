// Plain `?key=value` strings, as every link already sent out uses; the router's default JSON-encodes values.

export type SearchParams = Record<string, string | undefined>;

export function parseSearch(searchStr: string): SearchParams {
    const out: SearchParams = {};
    new URLSearchParams(searchStr).forEach((value, key) => {
        if (!(key in out)) out[key] = value;
    });
    return out;
}

export function stringifySearch(search: Record<string, unknown>): string {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(search)) {
        if (value === undefined || value === null || value === "") continue;
        params.set(key, String(value));
    }
    const str = params.toString();
    return str ? `?${str}` : "";
}

/** Splits a full href into link fields, for targets built elsewhere: `<Link {...hrefTarget(url)}>`. */
export function hrefTarget(href: string): { to: string; search: SearchParams; hash?: string } {
    const hashAt = href.indexOf("#");
    const hash = hashAt >= 0 ? href.slice(hashAt + 1) : undefined;
    const rest = hashAt >= 0 ? href.slice(0, hashAt) : href;
    const queryAt = rest.indexOf("?");
    const to = queryAt >= 0 ? rest.slice(0, queryAt) : rest;
    const search = queryAt >= 0 ? parseSearch(rest.slice(queryAt)) : {};
    return hash ? { to, search, hash } : { to, search };
}
