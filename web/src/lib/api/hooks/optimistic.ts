// Optimistic cache edits: patch before the request, undo on refusal, re-read once the last overlapping write settles.

import type { MutationKey, Query, QueryClient, QueryKey } from "@tanstack/react-query";

// Each entry: the key, what was there, and what the patch wrote (absent when nothing was written).
export type Snapshot = [QueryKey, unknown, unknown?][];

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

/** Cancels refetches under the keys and applies update to every loaded query there; returns what restoreQueries needs. */
export async function patchQueries(queryClient: QueryClient, keys: QueryKey[], update: Update): Promise<Snapshot> {
    // Only refetches: a cancelled first load would be left with nothing queued to retry it.
    await Promise.all(
        keys.map((queryKey) =>
            queryClient.cancelQueries({ queryKey, predicate: (query) => query.state.data !== undefined }),
        ),
    );
    const queries = loaded(queryClient, keys);
    const snapshot: Snapshot = [];
    for (const query of queries) {
        const previous = query.state.data;
        const next = update(previous, query.queryKey);
        if (next === previous) continue;
        queryClient.setQueryData(query.queryKey, next);
        snapshot.push([query.queryKey, previous, query.state.data]);
    }
    return snapshot;
}

/** Undoes the patch where the cache still holds it; anything newer is re-read instead of overwritten. */
export function restoreQueries(queryClient: QueryClient, snapshot: Snapshot | undefined) {
    for (const [queryKey, previous, written] of snapshot ?? []) {
        if (queryClient.getQueryData(queryKey) === written) queryClient.setQueryData(queryKey, previous);
        else void queryClient.invalidateQueries({ queryKey, exact: true });
    }
}

/** True while another mutation under the key is still in flight besides the caller. */
export function overlapping(queryClient: QueryClient, mutationKey: MutationKey): boolean {
    return queryClient.isMutating({ mutationKey }) > 1;
}

// Keys a settle deferred while other writes were pending, per mutation family.
const deferred = new Map<string, QueryKey[]>();

/** Re-reads the keys once no other write in the family is pending; the last one re-reads what the others deferred. */
export function settle(queryClient: QueryClient, mutationKey: MutationKey, keys: QueryKey[]) {
    const family = JSON.stringify(mutationKey);
    const pending = [...(deferred.get(family) ?? []), ...keys];
    if (overlapping(queryClient, mutationKey)) {
        deferred.set(family, pending);
        return;
    }
    deferred.delete(family);
    for (const queryKey of pending) void queryClient.invalidateQueries({ queryKey });
}

/** Applies fn to the rows of any cached list shape (array, page, infinite pages); a list fn leaves untouched keeps its identity. */
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
