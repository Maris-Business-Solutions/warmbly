import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { acquireDashboardImage, directDashboardImage } from "./dashboardImage";
import { SESSION_ENDED_EVENT } from "./auth";

const { request, token } = vi.hoisted(() => ({ request: vi.fn(), token: vi.fn() }));
vi.mock("@/lib/api/client/Request", () => ({ default: request }));
vi.mock("@/lib/helper/getToken", () => ({ default: token }));
vi.mock("@/lib/information", () => ({ API_URL: "https://api.example.test" }));

const nativeURL = URL;
const create = vi.fn(() => "blob:test-image");
const revoke = vi.fn();
const releases: (() => void)[] = [];
function acquire(src: string) {
    const lease = acquireDashboardImage(src);
    releases.push(lease.release);
    return lease;
}

beforeEach(() => {
    vi.clearAllMocks();
    token.mockReturnValue({ access_token: "synthetic-test-token" });
    request.mockResolvedValue(new Blob(["image"], { type: "image/png" }));
    vi.stubGlobal("URL", class extends nativeURL {
        static createObjectURL = create;
        static revokeObjectURL = revoke;
    });
});
afterEach(() => { releases.splice(0).forEach((release) => release()); vi.unstubAllGlobals(); });

describe("dashboard image previews", () => {
    it("keeps local, API, data and blob images direct", async () => {
        for (const source of ["/logo.svg", "https://api.example.test/public/avatars/a.png", "data:image/png;base64,AA==", "blob:existing-image"]) {
            expect(directDashboardImage(source)).toBe(true);
            expect(await acquire(source).promise).toBe(source);
        }
        expect(request).not.toHaveBeenCalled();
        expect(directDashboardImage("https://api.example.test.evil/image")).toBe(false);
        expect(directDashboardImage("https://user:password@api.example.test/image")).toBe(false);
        expect(directDashboardImage("javascript:alert(1)")).toBe(false);
    });

    it("fetches customer images with auth but never puts tokens or source URLs in the endpoint URL", async () => {
        const source = "https://customer.example/logo.png?signature=original";
        expect(await acquire(source).promise).toBe("blob:test-image");
        expect(request).toHaveBeenCalledWith(expect.objectContaining({ url: "/dashboard-image", authorization: true, data: { url: source }, responseType: "blob" }));
    });

    it("coalesces mounted images and revokes only after the last preview disappears", async () => {
        const first = acquire("https://cdn.example/shared.png");
        const second = acquire("https://cdn.example/shared.png");
        expect(await first.promise).toBe(await second.promise);
        expect(request).toHaveBeenCalledTimes(1);
        first.release();
        expect(revoke).not.toHaveBeenCalled();
        second.release();
        second.release();
        expect(revoke).toHaveBeenCalledTimes(1);
        expect(revoke).toHaveBeenCalledWith("blob:test-image");
    });

    it("uses the restricted public route before sign-in", async () => {
        token.mockReturnValue(null);
        await acquire("https://avatars.slack-edge.com/a.png").promise;
        expect(request).toHaveBeenCalledWith(expect.objectContaining({ url: "/dashboard-image/public", authorization: false }));
    });

    it("rejects non-image responses", async () => {
        request.mockResolvedValue(new Blob(["<html>"], { type: "text/html" }));
        await expect(acquire("https://cdn.example/fake.png").promise).rejects.toThrow("Not an image");
        expect(create).not.toHaveBeenCalled();
    });

    it("cancels an unmounted fetch without creating a blob URL", async () => {
        let resolve!: (blob: Blob) => void;
        request.mockReturnValue(new Promise<Blob>((done) => { resolve = done; }));
        const lease = acquire("https://cdn.example/pending.png");
        lease.release();
        const failed = expect(lease.promise).rejects.toMatchObject({ name: "AbortError" });
        resolve(new Blob(["image"], { type: "image/png" }));
        await failed;
        expect(create).not.toHaveBeenCalled();
    });

    it("clears session image bytes when the session ends", async () => {
        await acquire("https://cdn.example/session.png").promise;
        window.dispatchEvent(new Event(SESSION_ENDED_EVENT));
        expect(revoke).toHaveBeenCalledWith("blob:test-image");
        await acquire("https://cdn.example/session.png").promise;
        expect(request).toHaveBeenCalledTimes(2);
    });
});
