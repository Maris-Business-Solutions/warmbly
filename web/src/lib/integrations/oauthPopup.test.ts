// The integration connect popup: the window opens inside the click, before the
// authorization URL is fetched (Safari blocks it otherwise), and a blocked
// window still finishes once the person allows it from the address bar.

import { afterEach, describe, expect, it, vi } from "vitest";
import { authorizeInPopup } from "./oauthPopup";

vi.mock("react-hot-toast", () => ({ default: { error: vi.fn() } }));

function fakeWindow() {
    return {
        closed: false,
        location: { href: "about:blank", replace: vi.fn() },
        document: { title: "", body: { style: {}, innerHTML: "", childElementCount: 0 } },
        focus: vi.fn(),
        close: vi.fn(function (this: { closed: boolean }) {
            this.closed = true;
        }),
    };
}

function callback(data: Record<string, string>) {
    window.dispatchEvent(
        new MessageEvent("message", { origin: window.location.origin, data: { source: "warmbly-integration-oauth", ...data } }),
    );
}

afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
});

describe("authorizeInPopup", () => {
    it("opens the window before the start request resolves, then follows the URL", async () => {
        const popup = fakeWindow();
        const open = vi.spyOn(window, "open").mockReturnValue(popup as unknown as Window);
        let resolveStart!: (url: string) => void;
        const done = authorizeInPopup(() => new Promise<string>((r) => (resolveStart = r)));
        expect(open).toHaveBeenCalledTimes(1);
        expect(open.mock.calls[0][0]).toBe("");

        resolveStart("https://provider.example/authorize");
        await vi.waitFor(() => expect(popup.location.replace).toHaveBeenCalledWith("https://provider.example/authorize"));
        callback({ code: "c1", state: "s1" });
        await expect(done).resolves.toEqual({ code: "c1", state: "s1" });
        expect(popup.close).toHaveBeenCalled();
    });

    it("keeps listening when the browser blocks the window", async () => {
        const open = vi.spyOn(window, "open").mockReturnValue(null);
        const done = authorizeInPopup(async () => "https://provider.example/authorize");
        // The retry with the real address is what the blocked-window prompt opens.
        await vi.waitFor(() => expect(open).toHaveBeenCalledWith("https://provider.example/authorize", "warmbly_oauth", expect.any(String)));
        callback({ code: "c2", state: "s2" });
        await expect(done).resolves.toEqual({ code: "c2", state: "s2" });
    });

    it("rejects once the window is closed without an answer", async () => {
        vi.useFakeTimers();
        const popup = fakeWindow();
        vi.spyOn(window, "open").mockReturnValue(popup as unknown as Window);
        const done = authorizeInPopup(async () => "https://provider.example/authorize");
        const settled = expect(done).rejects.toThrow(/closed before finishing/);
        await vi.advanceTimersByTimeAsync(0);
        popup.closed = true;
        await vi.advanceTimersByTimeAsync(2000);
        await settled;
    });

    it("hands back a provider error", async () => {
        vi.spyOn(window, "open").mockReturnValue(fakeWindow() as unknown as Window);
        const done = authorizeInPopup(async () => "https://provider.example/authorize");
        const settled = expect(done).rejects.toThrow("access_denied");
        await Promise.resolve();
        await Promise.resolve();
        callback({ error: "access_denied" });
        await settled;
    });
});
