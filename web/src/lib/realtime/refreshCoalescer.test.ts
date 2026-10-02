import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { createDeliveryDeduper, createRefreshCoalescer } from "./refreshCoalescer";

describe("createRefreshCoalescer", () => {
    let client: QueryClient;

    beforeEach(() => {
        vi.useFakeTimers();
        client = new QueryClient();
    });
    afterEach(() => {
        vi.useRealTimers();
        client.clear();
    });

    it("turns a burst into one invalidation after it goes quiet", () => {
        const spy = vi.spyOn(client, "invalidateQueries");
        const c = createRefreshCoalescer(client, { delayMs: 1000, maxWaitMs: 5000 });
        for (let i = 0; i < 20; i++) {
            c.add([["unibox", "overview"], ["analytics"]]);
            vi.advanceTimersByTime(100);
        }
        expect(spy).not.toHaveBeenCalled();
        vi.advanceTimersByTime(1000);
        expect(spy).toHaveBeenCalledTimes(2);
    });

    it("still refreshes during a burst that never pauses", () => {
        const spy = vi.spyOn(client, "invalidateQueries");
        const c = createRefreshCoalescer(client, { delayMs: 1000, maxWaitMs: 3000 });
        for (let i = 0; i < 35; i++) {
            c.add([["unibox", "overview"]]);
            vi.advanceTimersByTime(100);
        }
        expect(spy).toHaveBeenCalledTimes(1);
        c.dispose();
    });

    it("follows a flush forced by the cap with a trailing one for keys added just before it", () => {
        const spy = vi.spyOn(client, "invalidateQueries");
        const c = createRefreshCoalescer(client, { delayMs: 1000, maxWaitMs: 3000 });
        c.add([["analytics"]]);
        for (let i = 0; i < 30; i++) {
            c.add([["unibox", "overview"]]);
            vi.advanceTimersByTime(100);
        }
        // The cap fired at 3000, 100ms after the last add.
        expect(spy.mock.calls.map((call) => call[0]?.queryKey)).toEqual([["analytics"], ["unibox", "overview"]]);
        vi.advanceTimersByTime(999);
        expect(spy).toHaveBeenCalledTimes(2);
        vi.advanceTimersByTime(1);
        expect(spy).toHaveBeenCalledTimes(3);
        expect(spy.mock.calls[2][0]).toMatchObject({ queryKey: ["unibox", "overview"] });
        vi.advanceTimersByTime(5000);
        expect(spy).toHaveBeenCalledTimes(3);
    });

    it("skips a key a broader pending key already covers", () => {
        const spy = vi.spyOn(client, "invalidateQueries");
        const c = createRefreshCoalescer(client, { delayMs: 10 });
        c.add([["unibox", "overview"], ["unibox"], ["unibox", "search"]]);
        vi.advanceTimersByTime(10);
        expect(spy).toHaveBeenCalledTimes(1);
        expect(spy.mock.calls[0][0]).toMatchObject({ queryKey: ["unibox"] });
    });

    it("leaves a running fetch alone and re-checks it next round", async () => {
        let resolve!: (v: number) => void;
        void client.fetchQuery({ queryKey: ["unibox", "overview"], queryFn: () => new Promise<number>((r) => (resolve = r)) });
        const spy = vi.spyOn(client, "invalidateQueries");
        const c = createRefreshCoalescer(client, { delayMs: 10 });
        c.add([["unibox", "overview"]]);
        vi.advanceTimersByTime(10);
        expect(spy).toHaveBeenCalledTimes(1);
        expect(client.getQueryState(["unibox", "overview"])?.isInvalidated).toBe(false);

        resolve(1);
        await vi.advanceTimersByTimeAsync(10);
        expect(spy).toHaveBeenCalledTimes(2);
        expect(client.getQueryState(["unibox", "overview"])?.isInvalidated).toBe(true);
    });
});

describe("createDeliveryDeduper", () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    it("drops the second copy of a delivery inside the window only", () => {
        const seen = createDeliveryDeduper(1000);
        expect(seen("EMAIL_RECEIVED|t1|m1")).toBe(false);
        expect(seen("EMAIL_RECEIVED|t1|m1")).toBe(true);
        expect(seen("EMAIL_UPDATED|t1|m1")).toBe(false);
        vi.advanceTimersByTime(1000);
        expect(seen("EMAIL_RECEIVED|t1|m1")).toBe(false);
    });
});
