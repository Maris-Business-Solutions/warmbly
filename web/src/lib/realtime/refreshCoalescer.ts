// Bounds realtime refetches: one invalidation per quiet interval (capped), and one handling per delivered event.

import { hashKey, partialMatchKey, type QueryClient, type QueryKey } from "@tanstack/react-query";

export interface RefreshCoalescer {
    /** Queues keys (prefixes) for the next flush. */
    add(keys: readonly QueryKey[]): void;
    /** Drops anything pending without invalidating it. */
    dispose(): void;
}

export interface RefreshCoalescerOptions {
    /** Quiet time after the last add before the flush. */
    delayMs?: number;
    /** Longest a key may wait while adds keep arriving. */
    maxWaitMs?: number;
}

// Kept above the backend's overviewFreshFor (internal/app/unibox/overview_cache.go).
export const REFRESH_DELAY_MS = 1500;
export const REFRESH_MAX_WAIT_MS = 5000;

export function createRefreshCoalescer(
    queryClient: QueryClient,
    { delayMs = REFRESH_DELAY_MS, maxWaitMs = REFRESH_MAX_WAIT_MS }: RefreshCoalescerOptions = {},
): RefreshCoalescer {
    // Each pending key with the time it was last added.
    const pending = new Map<string, { key: QueryKey; at: number }>();
    let timer: ReturnType<typeof setTimeout> | null = null;
    let firstAt = 0;

    const schedule = () => {
        const now = Date.now();
        if (timer) clearTimeout(timer);
        else firstAt = now;
        timer = setTimeout(flush, Math.max(0, Math.min(delayMs, firstAt + maxWaitMs - now)));
    };

    const add = (keys: readonly QueryKey[]) => {
        if (keys.length === 0) return;
        const now = Date.now();
        for (const key of keys) pending.set(hashKey(key), { key, at: now });
        schedule();
    };

    const flush = () => {
        timer = null;
        const now = Date.now();
        const entries = [...pending.values()];
        pending.clear();
        const keys = entries.map((e) => e.key);
        // A flush forced by the cap may beat the last event's writes, so recent keys get a trailing flush too.
        const again: QueryKey[] = entries.filter((e) => now - e.at < delayMs).map((e) => e.key);
        for (const key of keys) {
            // A broader pending key already covers this one.
            if (keys.some((other) => other !== key && other.length < key.length && partialMatchKey(key, other))) {
                continue;
            }
            // A running fetch finishes rather than restarts, and is re-checked next round since it may predate the event.
            for (const query of queryClient.getQueryCache().findAll({ queryKey: key, fetchStatus: "fetching" })) {
                again.push(query.queryKey);
            }
            void queryClient.invalidateQueries(
                { queryKey: key, predicate: (query) => query.state.fetchStatus !== "fetching" },
                { cancelRefetch: false },
            );
        }
        if (again.length > 0) add(again);
    };

    return {
        add,
        dispose: () => {
            if (timer) clearTimeout(timer);
            timer = null;
            pending.clear();
        },
    };
}

/**
 * Returns true when the same delivery was already seen within `windowMs`. The
 * key should name the event and its publish (type, timestamp, ids), so a
 * genuine second change is never mistaken for a copy.
 */
export function createDeliveryDeduper(windowMs = 2000): (key: string) => boolean {
    const seen = new Map<string, number>();
    return (key) => {
        const now = Date.now();
        // Insertion order is arrival order, so expired entries are at the front.
        for (const [k, at] of seen) {
            if (now - at < windowMs) break;
            seen.delete(k);
        }
        if (seen.has(key)) return true;
        seen.set(key, now);
        return false;
    };
}
