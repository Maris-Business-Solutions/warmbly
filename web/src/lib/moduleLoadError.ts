export function isModuleLoadError(error: unknown): boolean {
    if (!(error instanceof TypeError)) return false;
    if (/^(Failed to fetch dynamically imported module|error loading dynamically imported module)(:|$)/i.test(error.message)
        || error.message === "Importing a module script failed.") return true;

    const mime = error.message.match(/^['"]text\/html['"] is not a valid JavaScript MIME type for module script ['"]([^'"]+)['"]\.?$/);
    if (!mime || typeof window === "undefined") return false;
    try {
        const url = new URL(mime[1], window.location.href);
        return url.origin === window.location.origin && /^\/assets\/[^/]+\.js$/.test(url.pathname);
    } catch {
        return false;
    }
}
