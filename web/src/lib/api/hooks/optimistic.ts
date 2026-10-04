// Optimistic cache edits shared by the mutation hooks.
//
// A mutation patches every cached query that shows the entity before the
// request leaves, puts the snapshot back if the server refuses, and re-reads
// the same keys once the last overlapping write settles so the server's
// answer wins. The patchers understand every list shape the dashboard caches:
// a bare array, a { data, pagination } page, and infinite pages of either.

import type { MutationKey, Query, QueryClient, QueryKey } from "@tanstack/react-query";

export type Snapshot = [QueryKey, unknown][];

interface Row {
    id: string;
}

type Update = (data: unknown, queryKey: QueryKey) => unknown;

function isObject(value: unknown): value is Record<string, unknown> {
    return !!value && typeof value === "object" && !Array.isArray(value);
}

// Each cached query under any of the keys, once, skipping the ones still on a first load.
function loaded(queryClient: QueryClient, keys: QueryKey[]): Query[] {
    const seen = new Map<string, Query>();
    for (const queryKey of keys) {
        for (const query of queryClient.getQueryCache().findAll({ queryKey })) {
            if (query.state.data !== undefined) seen.set(query.queryHash, query);
        }
    }
    return [...seen.values()];
}

/**
 * Cancels in-flight refetches under the keys, snapshots every loaded query
 * there and applies update to each. Returns the snapshot for restoreQueries.
 */
export async function patchQueries(queryClient: QueryClient, keys: QueryKey[], update: Update): Promise<Snapshot> {
    // Only refetches: a cancelled first load would be left with nothing queued to retry it.
    await Promise.all(
        keys.map((queryKey) =>
            queryClient.cancelQueries({ queryKey, predicate: (query) => query.state.data !== undefined }),
        ),
    );
    const queries = loaded(queryClient, keys);
    const snapshot: Snapshot = queries.map((query) => [query.queryKey, query.state.data]);
    for (const query of queries) {
        const next = update(query.state.data, query.queryKey);
        if (next !== query.state.data) queryClient.setQueryData(query.queryKey, next);
    }
    return snapshot;
}

export function restoreQueries(queryClient: QueryClient, snapshot: Snapshot | undefined) {
    for (const [queryKey, data] of snapshot ?? []) queryClient.setQueryData(queryKey, data);
}

/** True while another mutation under the key is still in flight besides the caller. */
export function overlapping(queryClient: QueryClient, mutationKey: MutationKey): boolean {
    return queryClient.isMutating({ mutationKey }) > 1;
}

/**
 * Re-reads the keys once no other write under mutationKey is pending, so an
 * earlier answer cannot paint over a later optimistic edit.
 */
export function settle(queryClient: QueryClient, mutationKey: MutationKey, keys: QueryKey[]) {
    if (overlapping(queryClient, mutationKey)) return;
    for (const queryKey of keys) void queryClient.invalidateQueries({ queryKey });
}

/**
 * Applies fn to the rows of any cached list shape; anything else passes
 * through. A list fn leaves untouched keeps its identity.
 */
export function mapRows<T extends Row>(data: unknown, fn: (rows: T[]) => T[]): unknown {
    if (Array.isArray(data)) return fn(data as T[]);
    if (!isObject(data)) return data;
    if (Array.isArray(data.pages)) {
        const before = data.pages;
        const pages = before.map((page) => mapRows(page, fn));
        return pages.some((page, i) => page !== before[i]) ? { ...data, pages } : data;
    }
    if (Array.isArray(data.data)) {
        const rows = fn(data.data as T[]);
        return rows === data.data ? data : { ...data, data: rows };
    }
    return data;
}

// The rows with every match patched, or the same array when nothing matched.
function patchMatching<T extends Row>(rows: T[], match: (row: T) => boolean, patch: (row: T) => T): T[] {
    let hit = false;
    const next = rows.map((row) => {
        if (!row || !match(row)) return row;
        hit = true;
        return patch(row);
    });
    return hit ? next : rows;
}

/** Patches the entity with this id wherever it sits: a list row or a cached detail. */
export function updateEntity<T extends Row>(id: string, patch: (row: T) => T): Update {
    return updateEntities([id], patch);
}

/** Patches every entity whose id is in ids, list rows and cached details alike. */
export function updateEntities<T extends Row>(ids: Iterable<string>, patch: (row: T) => T): Update {
    const set = new Set(ids);
    return (data) => {
        if (isObject(data) && typeof data.id === "string" && set.has(data.id)) return patch(data as unknown as T);
        return mapRows<T>(data, (rows) => patchMatching(rows, (row) => set.has(row.id), patch));
    };
}

// The page's server total, less the rows taken off the cache.
function lowerTotal(page: unknown, removed: number): unknown {
    if (!isObject(page) || !isObject(page.pagination) || typeof page.pagination.total !== "number") return page;
    return { ...page, pagination: { ...page.pagination, total: Math.max(0, page.pagination.total - removed) } };
}

/** Takes the entities off every list, and lowers a server total by what left. */
export function removeEntities(ids: Iterable<string>): Update {
    const gone = new Set(ids);
    const keep = (rows: Row[]) => rows.filter((row) => !row || !gone.has(row.id));
    return (data) => {
        if (Array.isArray(data)) return keep(data as Row[]);
        if (!isObject(data)) return data;
        if (Array.isArray(data.pages)) {
            let removed = 0;
            const pages = data.pages.map((page) => {
                if (Array.isArray(page)) {
                    const kept = keep(page as Row[]);
                    removed += page.length - kept.length;
                    return kept;
                }
                if (!isObject(page) || !Array.isArray(page.data)) return page;
                const kept = keep(page.data as Row[]);
                removed += page.data.length - kept.length;
                return { ...page, data: kept };
            });
            // Every page carries the same whole-set total, so each loses the full count.
            return removed ? { ...data, pages: pages.map((page) => lowerTotal(page, removed)) } : data;
        }
        if (Array.isArray(data.data)) {
            const kept = keep(data.data as Row[]);
            const removed = data.data.length - kept.length;
            return removed ? lowerTotal({ ...data, data: kept }, removed) : data;
        }
        return data;
    };
}

/** The first cached copy of an entity under the keys, list row or detail. */
export function findEntity<T extends Row>(queryClient: QueryClient, keys: QueryKey[], id: string): T | undefined {
    let found: T | undefined;
    const look = (rows: T[]) => {
        found ??= rows.find((row) => row?.id === id);
        return rows;
    };
    for (const query of loaded(queryClient, keys)) {
        const data = query.state.data;
        if (isObject(data) && data.id === id) return data as unknown as T;
        mapRows<T>(data, look);
        if (found) return found;
    }
    return undefined;
}
