// Embed plumbing: the hosted page runs inside an iframe the /forms.js loader
// mounts, and reports its height so the frame never clips. Message names are
// the contract with internal/formserver/static/forms-embed.js.

export function isEmbedded(): boolean {
    return new URLSearchParams(window.location.search).get("embed") === "1";
}

// startResizeReporting posts the document height to the parent on every size
// change. No-op outside a frame. Returns a cleanup function.
export function startResizeReporting(publicId: string): () => void {
    if (window.parent === window) return () => {};
    const post = () => {
        try {
            window.parent.postMessage(
                { type: "warmbly:resize", form: publicId, height: document.documentElement.scrollHeight },
                "*",
            );
        } catch {
            // sandboxed parents can refuse messages; the frame just keeps its height
        }
    };
    post();
    const ro = new ResizeObserver(post);
    ro.observe(document.body);
    return () => ro.disconnect();
}

export function postSubmitted(publicId: string) {
    try {
        window.parent.postMessage({ type: "warmbly:submitted", form: publicId }, "*");
    } catch {
        // same as above
    }
}

// redirect escapes the iframe when possible so the thank-you page fills the
// tab, not a 480px frame. Only an absolute http(s) URL is followed; it reports
// whether it navigated.
export function redirect(raw: string): boolean {
    let url: string;
    try {
        const target = new URL(raw);
        if (target.protocol !== "http:" && target.protocol !== "https:") return false;
        url = target.href;
    } catch {
        return false;
    }
    try {
        if (window.top) {
            window.top.location.href = url;
            return true;
        }
    } catch {
        // cross-origin top blocks assignment; fall through to the frame itself
    }
    window.location.href = url;
    return true;
}
